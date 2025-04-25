package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func parseDateTime(input string) (time.Time, error) {
	parts := strings.Split(input, ",")
	parsed := [5]int{}
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("invalid input format")
	}

	for i, part := range parts {
		foo, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return time.Time{}, err
		}
		parsed[i] = foo
	}
	return time.Date(parsed[0], time.Month(parsed[1]), parsed[2], parsed[3], parsed[4], 0, 0, time.UTC), nil
}

// Styles
var s_year = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AD76E7"))
var s_dur = lipgloss.NewStyle().Italic(true).Bold(false).Foreground(lipgloss.Color("#3FC942"))
var s_chan = lipgloss.NewStyle().Italic(false).Bold(false).Foreground(lipgloss.Color("#EB9B19"))
var s_date = lipgloss.NewStyle().Italic(false).Bold(false).Foreground(lipgloss.Color("#797979"))

func atoi(s string, def int) int {
	x, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return x
}

func get_save_movie_file_name(epg SchedulableEPG) string {
	title := strings.ReplaceAll(epg.title, ` `, `_`)
	title = strings.ReplaceAll(title, ":", "_")
	title = strings.ReplaceAll(title, "?", "_")
	title = strings.ReplaceAll(title, "|", "_")
	return filepath.Join(dvb_rec_dir, fmt.Sprintf("%s-(%d).mts", title, epg.year))
}

func parse_recorded_files(db *sql.DB, rec_dir string) {
	year_rgx := regexp.MustCompile(`\(([0-9]{4})\)`)

	n := len(strings.Split(rec_dir, "/"))

	fmt.Printf("Searching for movies in %s", rec_dir)
	movie_files, err := filepath.Glob(fmt.Sprintf("/%s/*/*.mts", rec_dir))
	if err != nil {
		fmt.Printf("No movies found in %s", rec_dir)
		return
	}

	for _, movie_file := range movie_files {
		title := strings.Split(movie_file, "/")[n+1]
		title = strings.ReplaceAll(title, ".mts", "")
		year := year_rgx.FindStringSubmatch(title)
		if len(year) < 2 {
			continue
		}
		title = year_rgx.ReplaceAllString(title, "")
		fw_rec, err := get_fw_by_title_year(db, title, year[1])
		if err == nil && !fw_rec.recorded {
			fmt.Printf("Movie match  '%s'(%s) <-> '%s'(%d)! ... marking as recorded\n", title, year[1], fw_rec.title, fw_rec.year)
			mark_fw_as_recorded(db, fw_rec.id)
		}
	}
}
