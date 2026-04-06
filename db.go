package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/lib/pq"
)

var dbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7821D"))

// PostgreSQL error codes.
const (
	pqUniqueViolation     = "23505"
	pqForeignKeyViolation = "23503"
)

func isPQError(err error, code string) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == code
	}
	return false
}

// scanRows iterates over sql.Rows and scans each row using the provided function.
// It handles row closing and iteration errors uniformly.
func scanRows[T any](rows *sql.Rows, scanFn func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	var result []T
	for rows.Next() {
		item, err := scanFn(rows)
		if err != nil {
			log.Warn("failed to scan row", "err", err)
			continue
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("row iteration: %w", err)
	}
	return result, nil
}

func getChannels(db *sql.DB) ([]channel, error) {
	rows, err := db.Query(`SELECT "id","key","name" FROM "channel"`)
	if err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}
	return scanRows(rows, func(r *sql.Rows) (channel, error) {
		var ch channel
		err := r.Scan(&ch.id, &ch.key, &ch.name)
		return ch, err
	})
}

// getSchedulableEPGs returns future EPG entries that can be scheduled.
// Pass channelKey="all" to get entries for all channels.
func getSchedulableEPGs(db *sql.DB, channelKey string) ([]schedulableEPG, error) {
	baseQuery := `
SELECT
	epg.id,
	filmweb.title,
	filmweb.year,
	channel.name as channel,
	epg.start_time,
	epg.stop_time
FROM
	public.epg
	INNER JOIN filmweb ON epg.fw_id = filmweb.id
	INNER JOIN channel ON epg.channel_id = channel.id
WHERE
	filmweb.ignored = false
	AND epg.scheduled = false
	AND epg.start_time >= now()
	AND filmweb.recorded = false`

	var rows *sql.Rows
	var err error
	if channelKey != "all" {
		rows, err = db.Query(baseQuery+`
	AND channel.key = $1
ORDER BY filmweb.title ASC, channel.name ASC`, channelKey)
	} else {
		rows, err = db.Query(baseQuery + `
ORDER BY filmweb.title ASC, channel.name ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("get schedulable EPGs: %w", err)
	}
	return scanRows(rows, scanSchedulableEPG)
}

func getScheduledEPGs(db *sql.DB, onlyToday bool) ([]schedulableEPG, error) {
	where := `epg.scheduled = true AND epg.start_time >= now()`
	if onlyToday {
		where += ` AND epg.start_time <= current_date+1`
	}

	query := fmt.Sprintf(`
SELECT
	epg.id,
	filmweb.title,
	filmweb.year,
	channel.name as channel,
	epg.start_time,
	epg.stop_time
FROM
	public.epg
	INNER JOIN filmweb ON epg.fw_id = filmweb.id
	INNER JOIN channel ON epg.channel_id = channel.id
WHERE
	%s
ORDER BY
	filmweb.title ASC,
	epg.start_time,
	channel.name ASC`, where)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get scheduled EPGs: %w", err)
	}
	return scanRows(rows, scanSchedulableEPG)
}

// scanSchedulableEPG scans a basic schedulableEPG (id, title, year, channel, start, stop).
func scanSchedulableEPG(r *sql.Rows) (schedulableEPG, error) {
	var epg schedulableEPG
	err := r.Scan(&epg.id, &epg.title, &epg.year, &epg.channel, &epg.start, &epg.stop)
	return epg, err
}

func getChannelByKey(db *sql.DB, channelKey string) (channel, error) {
	var ch channel
	row := db.QueryRow(`SELECT * FROM channel WHERE key=$1`, channelKey)
	err := row.Scan(&ch.id, &ch.key, &ch.name)
	if errors.Is(err, sql.ErrNoRows) {
		return ch, fmt.Errorf("get channel by key: no channel with key %q", channelKey)
	}
	if err != nil {
		return ch, fmt.Errorf("get channel by key: %w", err)
	}
	return ch, nil
}

func fwEntryExists(db *sql.DB, id int) bool {
	var fwID int
	row := db.QueryRow("SELECT id FROM filmweb WHERE id = $1", id)
	if err := row.Scan(&fwID); err != nil {
		return false
	}
	return fwID > 0
}

func getIgnoredFilmwebIDs(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT id FROM filmweb WHERE ignored=true`)
	if err != nil {
		return nil, fmt.Errorf("get ignored filmweb IDs: %w", err)
	}
	items, err := scanRows(rows, func(r *sql.Rows) (int, error) {
		var id int
		return id, r.Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	result := make(map[int]bool, len(items))
	for _, id := range items {
		result[id] = true
	}
	return result, nil
}

func getIgnorableMovies(db *sql.DB) ([]movie, error) {
	rows, err := db.Query(`SELECT id, title, year FROM filmweb WHERE ignored=false ORDER BY title ASC, year ASC`)
	if err != nil {
		return nil, fmt.Errorf("get ignorable movies: %w", err)
	}
	return scanRows(rows, func(r *sql.Rows) (movie, error) {
		var m movie
		err := r.Scan(&m.id, &m.title, &m.year)
		return m, err
	})
}

func recordingsToStart(db *sql.DB, timeShiftMin int) ([]schedulableEPG, error) {
	q := `
SELECT
	epg.id,
	filmweb.title,
	filmweb.year,
	channel.name as channel,
	epg.start_time,
	epg.stop_time,
	epg.fw_id
FROM
	epg
	INNER JOIN filmweb ON epg.fw_id = filmweb.id
	INNER JOIN channel ON epg.channel_id = channel.id
WHERE
	epg.scheduled = true
	AND epg.recorder = $1
	AND epg.start_time - make_interval(mins => $2) <= now()
	AND epg.start_time + interval '10 second' >= now()
ORDER BY
	epg.start_time ASC`
	rows, err := db.Query(q, noRecorder, timeShiftMin)
	if err != nil {
		return nil, fmt.Errorf("recordings to start: %w", err)
	}
	return scanRows(rows, func(r *sql.Rows) (schedulableEPG, error) {
		var epg schedulableEPG
		err := r.Scan(&epg.id, &epg.title, &epg.year, &epg.channel, &epg.start, &epg.stop, &epg.fwID)
		return epg, err
	})
}

func recordingsToStop(db *sql.DB, timeShiftMin int) ([]schedulableEPG, error) {
	q := `
SELECT
	epg.id,
	filmweb.title,
	filmweb.year,
	filmweb.id,
	channel.name as channel,
	epg.start_time,
	epg.stop_time,
	epg.recorder,
	epg.pid
FROM
	epg
	INNER JOIN filmweb ON epg.fw_id = filmweb.id
	INNER JOIN channel ON epg.channel_id = channel.id
WHERE
	epg.scheduled = true
	AND epg.recorder > $1
	AND epg.stop_time + make_interval(mins => $2) <= now()
	AND epg.stop_time + make_interval(mins => $2) + interval '10 second' > now()
ORDER BY
	epg.start_time ASC`
	rows, err := db.Query(q, noRecorder, timeShiftMin)
	if err != nil {
		return nil, fmt.Errorf("recordings to stop: %w", err)
	}
	return scanRows(rows, func(r *sql.Rows) (schedulableEPG, error) {
		var epg schedulableEPG
		err := r.Scan(&epg.id, &epg.title, &epg.year, &epg.fwID, &epg.channel, &epg.start, &epg.stop, &epg.recorder, &epg.pid)
		return epg, err
	})
}

func getOccupiedRecorders(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT distinct(recorder) FROM epg WHERE recorder > $1`, noRecorder)
	if err != nil {
		return nil, fmt.Errorf("get occupied recorders: %w", err)
	}
	items, err := scanRows(rows, func(r *sql.Rows) (int, error) {
		var rec int
		return rec, r.Scan(&rec)
	})
	if err != nil {
		return nil, err
	}
	result := make(map[int]bool, len(items))
	for _, rec := range items {
		result[rec] = true
	}
	return result, nil
}

func addFilmwebEntry(db *sql.DB, fwID int, fwTitle string, fwYear int) {
	insertStm := `INSERT INTO "filmweb" ("id", "title", "year", "ignored", "recorded") VALUES($1, $2, $3, $4, $5)`
	_, err := db.Exec(insertStm, fwID, fwTitle, fwYear, false, false)
	if err != nil {
		if isPQError(err, pqUniqueViolation) {
			return
		}
		log.Warn("cannot add filmweb entry", "title", fwTitle, "year", fwYear, "err", err)
	}
}

func addEPGEntry(db *sql.DB, title string, year int, fwID int, channelID int, startTime time.Time, stopTime time.Time) {
	// Ensure the filmweb entry exists before inserting the EPG entry (FK constraint).
	if !fwEntryExists(db, fwID) {
		log.Debug("adding new filmweb entry", "title", title, "year", year)
		addFilmwebEntry(db, fwID, title, year)
	}

	insertStm := `INSERT INTO "epg" ("fw_id", "channel_id", "start_time", "stop_time", "scheduled", "recorder") VALUES($1, $2, $3, $4, $5, $6)`
	_, err := db.Exec(insertStm, fwID, channelID, startTime, stopTime, false, noRecorder)
	if err != nil {
		if isPQError(err, pqUniqueViolation) || isPQError(err, pqForeignKeyViolation) {
			return
		}
		log.Warn("cannot add EPG entry", "title", title, "fw_id", fwID, "err", err)
	}
}

func updateChannelEPG(db *sql.DB, ch channel) {
	epgs := getChannelEPG(ch.key)
	ignored, err := getIgnoredFilmwebIDs(db)
	if err != nil {
		log.Error("cannot get ignored filmweb IDs", "err", err)
		return
	}
	for _, epg := range epgs {
		if ignored[epg.filmID] {
			continue
		}
		startDatetime, err := parseDateTime(epg.start)
		if err != nil {
			log.Warn("cannot parse start time", "start", epg.start, "err", err)
			continue
		}
		stopDatetime, err := parseDateTime(epg.stop)
		if err != nil {
			log.Warn("cannot parse stop time", "stop", epg.stop, "err", err)
			continue
		}
		if startDatetime.After(time.Now()) {
			addEPGEntry(db, epg.title, epg.year, epg.filmID, ch.id, startDatetime, stopDatetime)
		}
	}
}

func updateEPGs(db *sql.DB, channelKey string) {
	channels, err := resolveChannels(db, channelKey)
	if err != nil {
		log.Error("cannot resolve channels", "key", channelKey, "err", err)
		return
	}
	for _, ch := range channels {
		log.Info("checking channel...", "name", ch.name)
		updateChannelEPG(db, ch)
	}
}

// resolveChannels returns a list of channels for the given key.
// If key is "all", all channels are returned; otherwise a single matching channel.
func resolveChannels(db *sql.DB, channelKey string) ([]channel, error) {
	if channelKey == "all" {
		return getChannels(db)
	}
	ch, err := getChannelByKey(db, channelKey)
	if err != nil {
		return nil, err
	}
	return []channel{ch}, nil
}

// setEPGScheduled sets the scheduled flag for an EPG entry.
func setEPGScheduled(db *sql.DB, epgID string, scheduled bool) error {
	_, err := db.Exec(`UPDATE epg SET scheduled = $1 WHERE id = $2`, scheduled, epgID)
	if err != nil {
		return fmt.Errorf("set EPG scheduled=%v id=%s: %w", scheduled, epgID, err)
	}
	return nil
}

func ignoreMovie(db *sql.DB, fwID string) error {
	_, err := db.Exec(`UPDATE filmweb SET ignored = true WHERE id = $1`, fwID)
	if err != nil {
		return fmt.Errorf("ignore movie id=%s: %w", fwID, err)
	}
	return nil
}

func markEPGBeingRecorded(db *sql.DB, epgID int, recorder int, pid int) error {
	_, err := db.Exec(`UPDATE epg SET recorder = $1, pid = $2 WHERE id = $3`, recorder, pid, epgID)
	if err != nil {
		return fmt.Errorf("mark EPG being recorded epg_id=%d: %w", epgID, err)
	}
	return nil
}

func markFWAsRecorded(db *sql.DB, fwID int) error {
	_, err := db.Exec(`UPDATE filmweb SET recorded = true WHERE id = $1`, fwID)
	if err != nil {
		return fmt.Errorf("mark filmweb recorded fw_id=%d: %w", fwID, err)
	}
	return nil
}

func markEPGNotRecorded(db *sql.DB, epgID int) error {
	_, err := db.Exec(`UPDATE epg SET recorder = $1, pid = 0, scheduled = false WHERE id = $2`, noRecorder, epgID)
	if err != nil {
		return fmt.Errorf("mark EPG not recorded epg_id=%d: %w", epgID, err)
	}
	return nil
}

func getFWByTitleYear(db *sql.DB, title string, year string) (fwEntry, error) {
	var fwrec fwEntry
	query := fmt.Sprintf(
		`SELECT id,title,year,ignored,recorded FROM filmweb WHERE SIMILARITY(title, $1) > %g AND year = $2`,
		similarityThreshold,
	)
	row := db.QueryRow(query, title, year)
	err := row.Scan(&fwrec.id, &fwrec.title, &fwrec.year, &fwrec.ignored, &fwrec.recorded)
	if err != nil {
		return fwrec, fmt.Errorf("get filmweb by title/year: %w", err)
	}
	return fwrec, nil
}

// vacuumStep defines a single vacuum operation.
type vacuumStep struct {
	query   string
	message string
}

var vacuumSteps = []vacuumStep{
	{
		query:   `DELETE FROM epg USING filmweb WHERE epg.fw_id = filmweb.id AND filmweb.ignored = true`,
		message: " - ignored movies removed from EPG",
	},
	{
		query:   `DELETE FROM epg USING filmweb WHERE epg.fw_id = filmweb.id AND filmweb.recorded = true`,
		message: " - already recorded movies removed from EPG",
	},
	{
		query:   `DELETE FROM epg WHERE epg.start_time < NOW() - INTERVAL '10 minutes'`,
		message: " - passed movies removed from EPG",
	},
}

func vacuum(db *sql.DB) {
	for _, step := range vacuumSteps {
		if _, err := db.Exec(step.query); err != nil {
			log.Warn("vacuum step failed", "msg", step.message, "err", err)
			continue
		}
		fmt.Println(dbStyle.Render(step.message))
	}
}

func dbInitConn() *sql.DB {
	psqlconn := os.ExpandEnv("$DVR_PG_CONN")
	if psqlconn == "" || psqlconn == "$DVR_PG_CONN" {
		log.Fatal("$DVR_PG_CONN environment variable is not set")
	}
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		log.Fatal("cannot connect to database, check $DVR_PG_CONN", "err", err)
	}
	// Configure connection pool for long-running watch mode.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db
}
