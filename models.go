package main

import (
	"time"
)

type EpgEntry struct {
	title    string
	year     int
	filmId   int
	sid      int
	start    string
	stop     string
	is_movie bool
}

type Channel struct {
	id   int
	key  string
	name string
}

type FwEntry struct {
	id       int
	title    string
	year     int
	ignored  bool
	recorded bool
}

type SchedulableEPG struct {
	id       int
	title    string
	year     int
	fwid     int
	channel  string
	start    time.Time
	stop     time.Time
	recorder int
	pid      int
}

func (e SchedulableEPG) duration() int {
	return int(e.stop.Sub(e.start).Minutes())
}

type Movie struct {
	id    int
	title string
	year  int
}
