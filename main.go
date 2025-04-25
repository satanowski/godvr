package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/log"
)

func pick_a_channel(db *sql.DB) string {
	var channels []huh.Option[string]
	var channel string
	for _, ch := range get_chanels(db) {
		channels = append(channels, huh.NewOption(ch.name, ch.key))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a channel").
				Options(channels...).
				Value(&channel),
		))

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}
	return channel
}

func get_epg(db *sql.DB, pick bool) {
	var channel string

	if pick {
		channel = pick_a_channel(db)
	} else {
		channel = "all"
	}
	update_epgs(db, channel)
}

func schedule(db *sql.DB, pick bool) {
	var channel_key string
	var events []huh.Option[string]
	var selected []string
	var max_title_len = 0

	if pick {
		channel_key = pick_a_channel(db)
	} else {
		channel_key = "all"
	}

	schedulable_epgs := get_schedulable_epgs(db, channel_key)
	// find longest title
	for _, epg := range schedulable_epgs {
		max_title_len = max(max_title_len, len(epg.title))
	}

	var s_title = lipgloss.NewStyle().Bold(true).Width(max_title_len + 1).Align(lipgloss.Left)

	for _, epg := range schedulable_epgs {
		duration := time.Time{}.Add(epg.stop.Sub(epg.start)).Format("3h04")
		date_time := strings.Split(epg.start.Format("2006.01.02 15:04"), " ")

		line := fmt.Sprintf(
			"%s (%s) %s %s + %s [%s]",
			s_title.Render(epg.title),
			s_year.Render(strconv.Itoa(epg.year)),
			s_date.Render(date_time[0]),
			date_time[1],
			s_dur.Render(duration),
			s_chan.Render(epg.channel),
		)
		events = append(events, huh.NewOption(line, strconv.Itoa(epg.id)))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Pick a movie").
				Options(events...).
				Value(&selected),
		))

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}
	for _, epg_id := range selected {
		log.Debug(fmt.Sprintf("Scheduling ID: %s...", epg_id))
		schedule_recording(db, epg_id)
	}
}

func unschedule(db *sql.DB) {
	var events []huh.Option[string]
	var selected []string
	var max_title_len = 0
	scheduled_epgs := get_scheduled_epgs(db, false)

	// find longest title
	for _, epg := range scheduled_epgs {
		max_title_len = max(max_title_len, len(epg.title))
	}
	var s_title = lipgloss.NewStyle().Bold(true).Width(max_title_len + 1).Align(lipgloss.Left)

	for _, epg := range scheduled_epgs {
		duration := time.Time{}.Add(epg.stop.Sub(epg.start)).Format("3h04")
		date_time := strings.Split(epg.start.Format("2006.01.02 15:04"), " ")

		line := fmt.Sprintf(
			"%s (%s) %s %s + %s [%s]",
			s_title.Render(epg.title),
			s_year.Render(strconv.Itoa(epg.year)),
			s_date.Render(date_time[0]),
			date_time[1],
			s_dur.Render(duration),
			s_chan.Render(epg.channel),
		)
		events = append(events, huh.NewOption(line, strconv.Itoa(epg.id)))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select movies to unschedule:").
				Options(events...).
				Value(&selected),
		))

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(selected)
	for _, epg_id := range selected {
		log.Debug(fmt.Sprintf("Unscheduling ID: %s...", epg_id))
		unschedule_recording(db, epg_id)
	}
}

func ignore(db *sql.DB) {
	var movies []huh.Option[string]
	var selected []string
	var max_title_len = 0
	ignorable_movies := get_ignorable_movies(db)

	// find longest title
	for _, epg := range ignorable_movies {
		max_title_len = max(max_title_len, len(epg.title))
	}
	var s_title = lipgloss.NewStyle().Bold(true).Width(max_title_len + 1).Align(lipgloss.Left)

	for _, epg := range ignorable_movies {
		line := fmt.Sprintf(
			"%s (%s)",
			s_title.Render(epg.title),
			s_year.Render(strconv.Itoa(epg.year)),
		)
		movies = append(movies, huh.NewOption(line, strconv.Itoa(epg.id)))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select movies to ingore:").
				Options(movies...).
				Value(&selected),
		))

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}
	for _, epg_id := range selected {
		log.Debug(fmt.Sprintf("Ignoring ID: %s...", epg_id))
		ignore_movie(db, epg_id)
	}
}

func list_scheduled(db *sql.DB, only_today bool) {
	l := list.New()
	max_title_len := 0
	scheduled_epgs := get_scheduled_epgs(db, only_today)

	// find longest title
	for _, epg := range scheduled_epgs {
		max_title_len = max(max_title_len, len(epg.title))
	}
	s_title := lipgloss.NewStyle().Bold(true).Width(max_title_len + 1).Align(lipgloss.Left)

	for _, epg := range scheduled_epgs {
		duration := time.Time{}.Add(epg.stop.Sub(epg.start)).Format("3h04")
		date_time := strings.Split(epg.start.Format("2006.01.02 15:04"), " ")
		line := fmt.Sprintf(
			"%s (%s) %s %s + %s [%s]",
			s_title.Render(epg.title),
			s_year.Render(strconv.Itoa(epg.year)),
			s_date.Render(date_time[0]),
			date_time[1],
			s_dur.Render(duration),
			s_chan.Render(epg.channel),
		)
		l.Item(line)
	}
	fmt.Println(l)
}

func watch(db *sql.DB) {
	if !dvb_adapters_present() {
		panic("No DVB adapters present!")
	}
	time_shift_before := atoi(os.ExpandEnv("$DVR_TIME_SHIFT_BEFORE"), 10)
	time_shift_after := atoi(os.ExpandEnv("$DVR_TIME_SHIFT_AFTER"), 15)
	sleep_time_int := atoi(os.ExpandEnv("$DVR_INTERVAL_SEC"), 10)
	sleep_time := time.Duration(sleep_time_int) * time.Second

	for {
		log.Info("Cleaning finished recordings if any...")
		for _, rec := range recordings_to_stop(db, time_shift_after) {
			log.Debug(fmt.Sprintf("Recording to check: %s (%d)...", rec.title, rec.year))
			if rec.pid > 0 && RecorderProcessExists(rec.pid) {
				log.Debug(fmt.Sprintf("PID %d exists, need to kill it...", rec.pid))
				// still working, need to kill it
				err := KillRecorderProcess(rec.pid + 1)
				if err != nil {
					log.Error(fmt.Sprintf("Cannot kill recorder process PID=%d", rec.pid+1))
				}
			}
			// Mark epg as recorded
			mark_epg_being_not_recorded(db, rec.id)
		}

		log.Info("Starting recordings if any...")
		for _, rec := range recordings_to_start(db, time_shift_before) {
			log.Debug(fmt.Sprintf("Recording to start: %s (%d)...", rec.title, rec.year))
			zap_cmd := PrepareZapCMD(db, rec, time_shift_after)
			if zap_cmd.adapter >= 0 {
				pid := RunAndForget(zap_cmd)
				if pid > 0 {
					mark_epg_being_recorded(db, rec.id, zap_cmd.adapter, pid)
					log.Debug(fmt.Sprintf("Recording started. PID=%d", pid))
				} else {
					log.Warn("Cannot start recording. Somethings wrong with execution!")
				}
			} else {
				log.Info("Cannot start recording. No free adapter available!")
			}
		}

		log.Info(fmt.Sprintf("Sleeping for %d seconds", int(sleep_time.Seconds())))
		fmt.Println()
		time.Sleep(sleep_time)
	}
}

func main() {

	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))
	log.SetLevel(log.DebugLevel)
	db := db_init_conn()
	defer db.Close()

	if len(os.Args) <= 1 {
		log.Error("Wrong execution!")
		os.Exit(1)
	}
	action := os.Args[1]
	switch action {
	case "e":
		fmt.Println(style.Render("EPG mode..."))
		get_epg(db, len(os.Args) == 3 && os.Args[2] == "p")
	case "s":
		fmt.Println(style.Render("Schedule for recording:"))
		schedule(db, len(os.Args) == 3 && os.Args[2] == "p")
	case "t":
		fmt.Println(style.Render("Scheduled for today:"))
		list_scheduled(db, true)
	case "p":
		fmt.Println(style.Render("All scheduled:"))
		list_scheduled(db, false)
	case "r":
		fmt.Println(style.Render("Unschedule:"))
		unschedule(db)
	case "i":
		fmt.Println(style.Render("Select movies to ignore:"))
		ignore(db)
	case "v":
		fmt.Println(style.Render("Vacuming..."))
		vacuming(db)
	case "w":
		fmt.Println(style.Render("Watching..."))
		watch(db)
	case "x":
		parse_recorded_files(db, os.ExpandEnv("$DVR_REC_DONE_DIR"))
	}
	fmt.Println(style.Render("...done"))
}
