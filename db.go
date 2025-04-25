package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	_ "github.com/lib/pq"
)

var db_style = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7821D"))

func get_chanels(db *sql.DB) []Channel {
	channel_keys := []Channel{}
	rows, err := db.Query(`SELECT "id","key","name" FROM "channel"`)
	if err != nil {
		panic(err)
	}

	defer rows.Close()
	for rows.Next() {
		var channel Channel
		err = rows.Scan(&channel.id, &channel.key, &channel.name)
		if err != nil {
			continue
		}
		channel_keys = append(channel_keys, channel)
	}
	return channel_keys
}

func get_schedulable_epgs(db *sql.DB, channel_key string) []SchedulableEPG {
	epgs := []SchedulableEPG{}
	qselect := `
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
	INNER JOIN channel ON epg.channel_id = channel.id`

	qwhere := "filmweb.ignored = false and epg.scheduled = false and epg.start_time >= now() and filmweb.recorded = false"
	if channel_key != "all" {
		qwhere = fmt.Sprintf("%s and channel.key = '%s'", qwhere, channel_key)
	}
	query := fmt.Sprintf("%s WHERE %s ORDER BY filmweb.title ASC, channel.name ASC", qselect, qwhere)

	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var epg SchedulableEPG
		err = rows.Scan(&epg.id, &epg.title, &epg.year, &epg.channel, &epg.start, &epg.stop)
		if err != nil {
			continue
		}
		epgs = append(epgs, epg)
	}
	return epgs
}

func get_scheduled_epgs(db *sql.DB, only_today bool) []SchedulableEPG {
	epgs := []SchedulableEPG{}
	qwhere := `epg.scheduled = true and epg.start_time >= now()`
	if only_today {
		qwhere = qwhere + `AND epg.start_time <= current_date+1`
	}

	qselect := `
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
	channel.name ASC`

	rows, err := db.Query(fmt.Sprintf(qselect, qwhere))
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		var epg SchedulableEPG
		err = rows.Scan(&epg.id, &epg.title, &epg.year, &epg.channel, &epg.start, &epg.stop)
		if err != nil {
			continue
		}
		epgs = append(epgs, epg)
	}
	return epgs
}

func get_channel_by_key(db *sql.DB, channel_key string) Channel {
	var channel Channel
	row := db.QueryRow(`SELECT * FROM channel WHERE key=$1`, channel_key)
	err := row.Scan(&channel.id, &channel.key, &channel.name)
	switch err {
	case sql.ErrNoRows:
		log.Error("No channel named: ", channel_key)
		return channel
	case nil:
		return channel
	default:
		panic(err)
	}
}

func fw_entry_exists(db *sql.DB, id int) bool {
	var fwId int
	row := db.QueryRow("SELECT id FROM filmweb WHERE id = ?", id)
	if err := row.Scan(&fwId); err != nil {
		if err == sql.ErrNoRows {
			return false
		}
	}
	return fwId > 0
}

func get_ignored_fwilmweb_ids(db *sql.DB) map[int]bool {
	ignored := map[int]bool{}

	rows, err := db.Query(`SELECT id FROM filmweb WHERE ignored=true`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var fw_id int
		err := rows.Scan(&fw_id)
		if err != nil {
			continue
		}
		ignored[fw_id] = true
	}
	return ignored
}

func get_ignorable_movies(db *sql.DB) []Movie {
	ignorable := []Movie{}
	rows, err := db.Query(`SELECT id, title, year FROM filmweb WHERE ignored=false ORDER BY title ASC, year ASC`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var movie Movie
		err := rows.Scan(&movie.id, &movie.title, &movie.year)
		if err != nil {
			continue
		}
		ignorable = append(ignorable, movie)
	}
	return ignorable
}

func recordings_to_start(db *sql.DB, time_shift_min int) []SchedulableEPG {
	to_start := []SchedulableEPG{}
	sql := `
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
	AND epg.recorder = -1
	AND epg.start_time - interval '%d minutes' <= now() 
	AND epg.start_time + interval '10 second' >= now()
ORDER BY
	epg.start_time ASC`
	rows, err := db.Query(fmt.Sprintf(sql, time_shift_min))
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var epg SchedulableEPG
		err := rows.Scan(&epg.id, &epg.title, &epg.year, &epg.channel, &epg.start, &epg.stop, &epg.fwid)
		if err != nil {
			continue
		}
		to_start = append(to_start, epg)
	}
	return to_start
}

func recordings_to_stop(db *sql.DB, time_shift_min int) []SchedulableEPG {
	to_stop := []SchedulableEPG{}
	sql := `
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
	AND epg.recorder > -1
	AND epg.stop_time + interval '%d minutes' <= now()
	AND epg.stop_time + interval '%d minutes 10second' > now()
ORDER BY
	epg.start_time ASC`
	rows, err := db.Query(fmt.Sprintf(sql, time_shift_min, time_shift_min))
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var epg SchedulableEPG
		err := rows.Scan(
			&epg.id,
			&epg.title,
			&epg.year,
			&epg.fwid,
			&epg.channel,
			&epg.start,
			&epg.stop,
			&epg.recorder,
			&epg.pid,
		)
		if err != nil {
			continue
		}
		to_stop = append(to_stop, epg)
	}
	return to_stop
}

func get_occupied_recorders(db *sql.DB) map[int]bool {
	results := map[int]bool{}
	rows, err := db.Query(`SELECT distinct(recorder) FROM epg WHERE recorder > -1`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var rec int
		if rows.Scan(&rec) != nil {
			continue
		}
		results[rec] = true
	}
	return results
}

func add_filmweb_entry(db *sql.DB, fw_id int, fw_title string, fw_year int) {
	insert_stm := `insert into "filmweb" ("id", "title", "year", "ignored", "recorded") values($1, $2, $3, $4, $5)`
	_, err := db.Exec(
		insert_stm,
		fw_id,
		fw_title,
		fw_year,
		false,
		false,
	)
	if err != nil {
		if strings.Contains(err.Error(), "epg_fw_id_start_time_idx") || strings.Contains(err.Error(), "filmweb_pkey") {
			// duplicated field, ignore
		} else {
			log.Warn(fmt.Sprintf("Cannot add Filmweb entry: %s (%d):\t %v", fw_title, fw_year, err))
		}
	}
}

func add_epg_entry(db *sql.DB, title string, year int, fw_id int, channel_id int, start_time time.Time, stop_time time.Time) {
	insert_stm := `insert into "epg" ("fw_id", "channel_id", "start_time", "stop_time", "scheduled", "recorder") values($1, $2, $3, $4, $5, $6)`
	_, err := db.Exec(
		insert_stm,
		fw_id,
		channel_id,
		start_time,
		stop_time,
		false,
		-1,
	)
	if err != nil {
		if strings.Contains(err.Error(), "epg_fw_id_fkey") || strings.Contains(err.Error(), "epg_fw_id_start_time_idx") {
		} else {
			log.Warn(fmt.Sprintf("Cannot add EPG entry: %s (%d):\t %v", title, fw_id, err))
		}
	}
	if !fw_entry_exists(db, fw_id) {
		log.Debug(fmt.Sprintf("Adding new FilmwebEntry: '%s' (%d)", title, year))
		add_filmweb_entry(db, fw_id, title, year)
	}

}

func update_channel_epg(db *sql.DB, channel Channel) {
	epgs := get_channel_epg(channel.key)
	ignored := get_ignored_fwilmweb_ids(db)
	for _, epg := range epgs {

		_, ok := ignored[epg.filmId]
		if ok {
			//log.Debug(fmt.Sprintf("Movie '%s' is ignored", epg.title))
			continue
		}
		start_datetime, err := parseDateTime(epg.start)
		if err != nil {
			log.Warn(fmt.Sprintf("Cannot parse start time: '%s' ", epg.start))
			continue
		}
		stop_datetime, err := parseDateTime(epg.stop)
		if err != nil {
			log.Warn(fmt.Sprintf("Cannot parse stop time: '%s' ", epg.stop))
			continue
		}
		if start_datetime.After(time.Now()) {
			add_epg_entry(db, epg.title, epg.year, epg.filmId, channel.id, start_datetime, stop_datetime)
		}
	}
}

func update_epgs(db *sql.DB, channel_key string) {
	var channels []Channel

	if channel_key == "all" {
		channels = get_chanels(db)
	} else {
		channels = []Channel{get_channel_by_key(db, channel_key)}
	}

	for _, channel := range channels {
		log.Info(fmt.Sprintf("Checking %s...", channel.name))
		update_channel_epg(db, channel)
	}

}

func schedule_recording(db *sql.DB, epg_id string) {
	_, err := db.Exec(`UPDATE epg SET scheduled = true WHERE id = $1;`, epg_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot schedule EPG.ID = %s", epg_id))
	}
}

func unschedule_recording(db *sql.DB, epg_id string) {
	_, err := db.Exec(`UPDATE epg SET scheduled = false WHERE id = $1;`, epg_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot schedule EPG.ID = %s", epg_id))
	}
}

func ignore_movie(db *sql.DB, fw_id string) {
	_, err := db.Exec(`UPDATE filmweb SET ignored = true WHERE id = $1;`, fw_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot ignore movie with ID = %s", fw_id))
	}
}

func mark_epg_being_recorded(db *sql.DB, epg_id int, recorder int, pid int) {
	_, err := db.Exec(`UPDATE epg SET recorder = $1, pid = $2 WHERE id = $3;`, recorder, pid, epg_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot mark EPG %d as being recorded (adapter: %d, pid: %d)!", epg_id, recorder, pid))
	}
}

func mark_fw_as_recorded(db *sql.DB, fw_id int) {
	_, err := db.Exec(`UPDATE filmweb SET recorded = true WHERE id = $1;`, fw_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot mark FW movie with ID = %d as recorded", fw_id))
	}
}

func mark_epg_being_not_recorded(db *sql.DB, epg_id int) {
	_, err := db.Exec(`UPDATE epg SET recorder = -1, pid =0, scheduled = false WHERE id = $1;`, epg_id)
	if err != nil {
		log.Warn(fmt.Sprintf("Cannot mark EPG %d as being not recorded!", epg_id))
	}
}

func get_fw_by_title_year(db *sql.DB, title string, year string) (FwEntry, error) {
	var fwrec FwEntry
	query := `SELECT id,title,year,ignored, recorded FROM filmweb WHERE SIMILARITY(title, $1) > 0.6 and year = $2;`
	row := db.QueryRow(query, title, year)
	err := row.Scan(&fwrec.id, &fwrec.title, &fwrec.year, &fwrec.ignored, &fwrec.recorded)
	return fwrec, err
}

func vacuming(db *sql.DB) {
	// remove EPGs related to ignored movies
	_, err := db.Exec(`DELETE FROM epg USING filmweb WHERE epg.fw_id = filmweb.id AND filmweb.ignored = true;`)
	if err != nil {
		log.Warn("Cannot remove ignored EPGs")
	} else {
		fmt.Println(db_style.Render(" - ignored movies removed from EPG"))
	}

	// remove EPGs of movies that are allready recorded
	_, err = db.Exec(`DELETE FROM epg USING filmweb WHERE epg.fw_id = filmweb.id AND filmweb.recorded = true;`)
	if err != nil {
		log.Warn("Cannot remove ignored EPGs")
	} else {
		fmt.Println(db_style.Render(" - already recorded movies removed from EPG"))
	}

	// remove old EPGs
	_, err = db.Exec(`DELETE FROM epg WHERE EPG.START_TIME < NOW() - INTERVAL '10 minutes'`)
	if err != nil {
		log.Warn("Cannot remove old epgs EPGs")
	} else {
		fmt.Println(db_style.Render(" - passed movies removed from EPG"))
	}

}

func db_init_conn() *sql.DB {
	psqlconn := os.ExpandEnv("$DVR_PG_CONN")
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		log.Error(fmt.Sprintf("Cannot connect to database: %s !\n", psqlconn))
		panic(err)
	}
	return db
}
