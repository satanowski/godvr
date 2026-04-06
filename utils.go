package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// parseDateTime parses an ISO 8601 datetime string (e.g. "2026-04-06T22:30:00").
func parseDateTime(input string) (time.Time, error) {
	t, err := time.Parse("2006-01-02T15:04:05", input)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse datetime %q: %w", input, err)
	}
	return t, nil
}

// TUI styles for formatting EPG display lines.
var (
	yearStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AD76E7"))
	durStyle  = lipgloss.NewStyle().Italic(true).Bold(false).Foreground(lipgloss.Color("#3FC942"))
	chanStyle = lipgloss.NewStyle().Italic(false).Bold(false).Foreground(lipgloss.Color("#EB9B19"))
	dateStyle = lipgloss.NewStyle().Italic(false).Bold(false).Foreground(lipgloss.Color("#797979"))
)

func atoi(s string, def int) int {
	x, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return x
}

// maxTitleLen returns the length of the longest title in the given EPG entries.
func maxTitleLen(epgs []schedulableEPG) int {
	maxLen := 0
	for _, epg := range epgs {
		maxLen = max(maxLen, len(epg.title))
	}
	return maxLen
}

// formatEPGLine formats a single EPG entry into a styled display string.
func formatEPGLine(epg schedulableEPG, titleStyle lipgloss.Style) string {
	duration := time.Time{}.Add(epg.stop.Sub(epg.start)).Format("3h04")
	dateTime := strings.Split(epg.start.Format("2006.01.02 15:04"), " ")
	return fmt.Sprintf(
		"%s (%s) %s %s + %s [%s]",
		titleStyle.Render(epg.title),
		yearStyle.Render(strconv.Itoa(epg.year)),
		dateStyle.Render(dateTime[0]),
		dateTime[1],
		durStyle.Render(duration),
		chanStyle.Render(epg.channel),
	)
}

// epgToOptions converts EPG entries into huh.Option items using formatEPGLine.
func epgToOptions(epgs []schedulableEPG) []huh.Option[string] {
	titleStyle := lipgloss.NewStyle().Bold(true).Width(maxTitleLen(epgs) + 1).Align(lipgloss.Left)
	options := make([]huh.Option[string], 0, len(epgs))
	for _, epg := range epgs {
		line := formatEPGLine(epg, titleStyle)
		options = append(options, huh.NewOption(line, strconv.Itoa(epg.id)))
	}
	return options
}

// runMultiSelect presents a multi-select form with the given title and options,
// returning the selected values.
func runMultiSelect(title string, options []huh.Option[string]) ([]string, error) {
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(title).
				Options(options...).
				Value(&selected),
		))
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("form: %w", err)
	}
	return selected, nil
}

// safeFilenameRgx matches only characters safe for filenames (alphanumeric, dash, underscore, dot, parens).
var safeFilenameRgx = regexp.MustCompile(`[^a-zA-Z0-9._()-]`)

func getSafeMovieFileName(epg schedulableEPG) string {
	title := safeFilenameRgx.ReplaceAllString(epg.title, "_")
	return filepath.Join(dvbRecDir, fmt.Sprintf("%s-(%d).mts", title, epg.year))
}

func parseRecordedFiles(db *sql.DB, recDir string) {
	yearRgx := regexp.MustCompile(`\(([0-9]{4})\)`)

	n := len(strings.Split(recDir, "/"))

	log.Info("Searching for movies", "dir", recDir)
	movieFiles, err := filepath.Glob(fmt.Sprintf("/%s/*/*.mts", recDir))
	if err != nil {
		log.Warn("No movies found", "dir", recDir)
		return
	}

	for _, movieFile := range movieFiles {
		title := strings.Split(movieFile, "/")[n+1]
		title = strings.ReplaceAll(title, ".mts", "")
		year := yearRgx.FindStringSubmatch(title)
		if len(year) < 2 {
			continue
		}
		title = yearRgx.ReplaceAllString(title, "")
		fwRec, err := getFWByTitleYear(db, title, year[1])
		if err == nil && !fwRec.recorded {
			log.Info("Movie match, marking as recorded",
				"file_title", title, "file_year", year[1],
				"db_title", fwRec.title, "db_year", fwRec.year)
			markFWAsRecorded(db, fwRec.id)
		}
	}
}
