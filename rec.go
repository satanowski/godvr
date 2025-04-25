package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

var dvb_front = atoi(os.ExpandEnv("$DVR_DVB_FRONT"), 1)
var dvb_lna = atoi(os.ExpandEnv("$DVR_DVB_LNA"), 1)
var dvb_dongles_count = atoi(os.ExpandEnv("$DVR_DVB_DONGLES_COUNT"), 1)
var dvb_channels_file = os.ExpandEnv("$DVR_CHANNELS_FILE")
var dvb_rec_dir = os.ExpandEnv("$DVR_REC_DIR")

const dvb_zap string = "/usr/bin/dvbv5-zap"

func dvb_adapters_present() bool {
	matches, _ := filepath.Glob(fmt.Sprintf("/dev/dvb/*/frontend%d", dvb_front))
	return len(matches) > 0
}

func get_free_recorder(db *sql.DB) int {
	occupied_recorders := get_occupied_recorders(db)
	for i := range dvb_dongles_count {
		_, ok := occupied_recorders[i]
		if !ok { // i-th recorder not occupied
			return i
		}
	}
	return -1
}

type ZapCmd struct {
	cmd     string
	args    []string
	adapter int
}

func PrepareZapCMD(db *sql.DB, epg SchedulableEPG, time_shift_min int) ZapCmd {
	adapter := get_free_recorder(db)
	if adapter < 0 {
		return ZapCmd{adapter: -1}
	}

	return ZapCmd{
		cmd: "/usr/bin/timeout",
		args: []string{
			"-s", "9",
			strconv.Itoa((epg.duration() * 60) + time_shift_min),
			dvb_zap,
			"-a", strconv.Itoa(adapter),
			"-f", strconv.Itoa(dvb_front),
			fmt.Sprintf("--lna=%d", dvb_lna),
			"-c", dvb_channels_file,
			epg.channel,
			"-r", "-o", get_save_movie_file_name(epg),
		},
		adapter: adapter,
	}
}

func RunAndForget(zap_cmd ZapCmd) int {
	cmd := exec.Command(zap_cmd.cmd, zap_cmd.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start command: %v", err)
		return 0
	}
	return cmd.Process.Pid
}

func RecorderProcessExists(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}

func KillRecorderProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = process.Kill()
	if err != nil {
		return err
	}
	return nil
}
