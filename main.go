package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/log"
)

func pickChannel(db *sql.DB) string {
	var ch string
	chs, err := getChannels(db)
	if err != nil {
		log.Fatal("cannot get channels", "err", err)
	}
	channels := make([]huh.Option[string], 0, len(chs))
	for _, c := range chs {
		channels = append(channels, huh.NewOption(c.name, c.key))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a channel").
				Options(channels...).
				Value(&ch),
		))
	if err := form.Run(); err != nil {
		log.Fatal(err)
	}
	return ch
}

// resolveChannelKey returns the channel key from user selection or "all".
func resolveChannelKey(db *sql.DB, pick bool) string {
	if pick {
		return pickChannel(db)
	}
	return "all"
}

func getEPG(db *sql.DB, pick bool) {
	updateEPGs(db, resolveChannelKey(db, pick))
}

func schedule(db *sql.DB, pick bool) {
	channelKey := resolveChannelKey(db, pick)

	schedulableEpgs, err := getSchedulableEPGs(db, channelKey)
	if err != nil {
		log.Error("cannot get schedulable EPGs", "err", err)
		return
	}

	selected, err := runMultiSelect("Pick a movie", epgToOptions(schedulableEpgs))
	if err != nil {
		log.Fatal(err)
	}
	for _, epgID := range selected {
		log.Debug("scheduling", "id", epgID)
		if err := setEPGScheduled(db, epgID, true); err != nil {
			log.Warn("cannot schedule", "id", epgID, "err", err)
		}
	}
}

func unschedule(db *sql.DB) {
	scheduledEpgs, err := getScheduledEPGs(db, false)
	if err != nil {
		log.Error("cannot get scheduled EPGs", "err", err)
		return
	}

	selected, err := runMultiSelect("Select movies to unschedule:", epgToOptions(scheduledEpgs))
	if err != nil {
		log.Fatal(err)
	}
	for _, epgID := range selected {
		log.Debug("unscheduling", "id", epgID)
		if err := setEPGScheduled(db, epgID, false); err != nil {
			log.Warn("cannot unschedule", "id", epgID, "err", err)
		}
	}
}

func ignore(db *sql.DB) {
	ignorableMovies, err := getIgnorableMovies(db)
	if err != nil {
		log.Error("cannot get ignorable movies", "err", err)
		return
	}

	// Build options from movies.
	mtl := 0
	for _, m := range ignorableMovies {
		mtl = max(mtl, len(m.title))
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Width(mtl + 1).Align(lipgloss.Left)

	options := make([]huh.Option[string], 0, len(ignorableMovies))
	for _, m := range ignorableMovies {
		line := fmt.Sprintf(
			"%s (%s)",
			titleStyle.Render(m.title),
			yearStyle.Render(strconv.Itoa(m.year)),
		)
		options = append(options, huh.NewOption(line, strconv.Itoa(m.id)))
	}

	selected, err := runMultiSelect("Select movies to ignore:", options)
	if err != nil {
		log.Fatal(err)
	}
	for _, fwID := range selected {
		log.Debug("ignoring", "id", fwID)
		if err := ignoreMovie(db, fwID); err != nil {
			log.Warn("cannot ignore movie", "id", fwID, "err", err)
		}
	}
}

func listScheduled(db *sql.DB, onlyToday bool) {
	l := list.New()
	scheduledEpgs, err := getScheduledEPGs(db, onlyToday)
	if err != nil {
		log.Error("cannot get scheduled EPGs", "err", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Width(maxTitleLen(scheduledEpgs) + 1).Align(lipgloss.Left)
	for _, epg := range scheduledEpgs {
		l.Item(formatEPGLine(epg, titleStyle))
	}
	fmt.Println(l)
}

func watch(db *sql.DB) {
	if !dvbAdaptersPresent() {
		log.Fatal("no DVB adapters present")
	}
	timeShiftBefore := atoi(os.ExpandEnv("$DVR_TIME_SHIFT_BEFORE"), defaultTimeShiftBefore)
	timeShiftAfter := atoi(os.ExpandEnv("$DVR_TIME_SHIFT_AFTER"), defaultTimeShiftAfter)
	sleepTimeInt := atoi(os.ExpandEnv("$DVR_INTERVAL_SEC"), defaultIntervalSec)
	sleepTime := time.Duration(sleepTimeInt) * time.Second

	// Set up signal handling for graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for {
		stopFinishedRecordings(db, timeShiftAfter)
		startDueRecordings(db, timeShiftBefore, timeShiftAfter)

		log.Info("sleeping", "seconds", int(sleepTime.Seconds()))
		fmt.Println()

		select {
		case <-ctx.Done():
			log.Info("received shutdown signal, cleaning up...")
			// TODO: kill running recordings gracefully
			return
		case <-time.After(sleepTime):
			// continue loop
		}
	}
}

// stopFinishedRecordings kills recorder processes for EPGs that have finished.
func stopFinishedRecordings(db *sql.DB, timeShiftAfter int) {
	log.Info("cleaning finished recordings if any...")
	recsToStop, err := recordingsToStop(db, timeShiftAfter)
	if err != nil {
		log.Error("cannot query recordings to stop", "err", err)
		return
	}
	for _, rec := range recsToStop {
		log.Debug("recording to check", "title", rec.title, "year", rec.year)
		if rec.pid > 0 && recorderProcessExists(rec.pid) {
			log.Debug("PID exists, killing process group", "pid", rec.pid)
			if err := killRecorderProcess(rec.pid); err != nil {
				log.Error("cannot kill recorder process group", "pid", rec.pid, "err", err)
			}
		}
		if err := markEPGNotRecorded(db, rec.id); err != nil {
			log.Warn("cannot mark EPG not recorded", "id", rec.id, "err", err)
		}
	}
}

// startDueRecordings starts recorder processes for EPGs that are due.
func startDueRecordings(db *sql.DB, timeShiftBefore, timeShiftAfter int) {
	log.Info("starting recordings if any...")
	recsToStart, err := recordingsToStart(db, timeShiftBefore)
	if err != nil {
		log.Error("cannot query recordings to start", "err", err)
		return
	}
	for _, rec := range recsToStart {
		log.Debug("recording to start", "title", rec.title, "year", rec.year)
		zc, err := prepareZapCmd(db, rec, timeShiftAfter)
		if err != nil {
			log.Error("cannot prepare recording command", "title", rec.title, "err", err)
			continue
		}
		if zc.adapter < 0 {
			log.Info("cannot start recording, no free adapter available")
			continue
		}
		pid, err := runAndForget(zc)
		if err != nil {
			log.Error("cannot start recording", "title", rec.title, "err", err)
			continue
		}
		if pid > 0 {
			if err := markEPGBeingRecorded(db, rec.id, zc.adapter, pid); err != nil {
				log.Warn("cannot mark EPG being recorded", "id", rec.id, "err", err)
			}
			log.Debug("recording started", "pid", pid)
		}
	}
}

func main() {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))
	log.SetLevel(log.DebugLevel)
	db := dbInitConn()
	defer db.Close()

	if len(os.Args) <= 1 {
		log.Error("no action specified")
		os.Exit(1)
	}

	pick := len(os.Args) == 3 && os.Args[2] == "p"

	switch os.Args[1] {
	case "e":
		fmt.Println(style.Render("EPG mode..."))
		getEPG(db, pick)
	case "s":
		fmt.Println(style.Render("Schedule for recording:"))
		schedule(db, pick)
	case "t":
		fmt.Println(style.Render("Scheduled for today:"))
		listScheduled(db, true)
	case "p":
		fmt.Println(style.Render("All scheduled:"))
		listScheduled(db, false)
	case "r":
		fmt.Println(style.Render("Unschedule:"))
		unschedule(db)
	case "i":
		fmt.Println(style.Render("Select movies to ignore:"))
		ignore(db)
	case "v":
		fmt.Println(style.Render("Vacuuming..."))
		vacuum(db)
	case "w":
		fmt.Println(style.Render("Watching..."))
		watch(db)
	case "x":
		parseRecordedFiles(db, os.ExpandEnv("$DVR_REC_DONE_DIR"))
	}
	fmt.Println(style.Render("...done"))
}
