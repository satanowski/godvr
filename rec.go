package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"github.com/charmbracelet/log"
)

var dvbFront = atoi(os.ExpandEnv("$DVR_DVB_FRONT"), 1)
var dvbLNA = atoi(os.ExpandEnv("$DVR_DVB_LNA"), 1)
var dvbDonglesCount = atoi(os.ExpandEnv("$DVR_DVB_DONGLES_COUNT"), 1)
var dvbChannelsFile = os.ExpandEnv("$DVR_CHANNELS_FILE")
var dvbRecDir = os.ExpandEnv("$DVR_REC_DIR")

const dvbZap string = "/usr/bin/dvbv5-zap"

// validChannelName matches only safe channel identifiers (alphanumeric, dashes, underscores, dots, spaces)
var validChannelName = regexp.MustCompile(`^[a-zA-Z0-9 _.\-]+$`)

func dvbAdaptersPresent() bool {
	matches, _ := filepath.Glob(fmt.Sprintf("/dev/dvb/*/frontend%d", dvbFront))
	return len(matches) > 0
}

func getFreeRecorder(db *sql.DB) int {
	occupiedRecorders, err := getOccupiedRecorders(db)
	if err != nil {
		log.Error("Cannot get occupied recorders", "err", err)
		return noRecorder
	}
	for i := range dvbDonglesCount {
		_, ok := occupiedRecorders[i]
		if !ok { // i-th recorder not occupied
			return i
		}
	}
	return noRecorder
}

type zapCmd struct {
	cmd     string
	args    []string
	adapter int
	pgid    bool // whether process group was set
}

func prepareZapCmd(db *sql.DB, epg schedulableEPG, timeShiftMin int) (zapCmd, error) {
	// Validate channel name to prevent command argument injection
	if !validChannelName.MatchString(epg.channel) {
		return zapCmd{adapter: noRecorder}, fmt.Errorf("invalid channel name: %q", epg.channel)
	}

	adapter := getFreeRecorder(db)
	if adapter < 0 {
		return zapCmd{adapter: noRecorder}, nil
	}

	return zapCmd{
		cmd: "/usr/bin/timeout",
		args: []string{
			"-s", killSignal,
			strconv.Itoa((epg.duration() * 60) + timeShiftMin),
			dvbZap,
			"-a", strconv.Itoa(adapter),
			"-f", strconv.Itoa(dvbFront),
			fmt.Sprintf("--lna=%d", dvbLNA),
			"-c", dvbChannelsFile,
			epg.channel,
			"-r", "-o", getSafeMovieFileName(epg),
		},
		adapter: adapter,
		pgid:    true,
	}, nil
}

func runAndForget(zc zapCmd) (int, error) {
	cmd := exec.Command(zc.cmd, zc.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Create a new process group so we can kill the entire tree later
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err := cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start recording command: %w", err)
	}
	return cmd.Process.Pid, nil
}

func recorderProcessExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check if process is alive.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func killRecorderProcess(pid int) error {
	// Kill the entire process group (negative PID) to ensure child processes are also terminated
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if err != nil {
		return fmt.Errorf("failed to kill process group for PID %d: %w", pid, err)
	}
	return nil
}
