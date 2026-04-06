package main

import (
	"time"
)

// noRecorder is the sentinel value indicating no DVB adapter is assigned.
const noRecorder = -1

// similarityThreshold is the minimum pg_trgm similarity score for title matching.
const similarityThreshold = 0.6

// killSignal is the signal number used by timeout to kill dvbv5-zap.
const killSignal = "9"

// defaultTimeShiftBefore is the default minutes to start recording before EPG start time.
const defaultTimeShiftBefore = 10

// defaultTimeShiftAfter is the default minutes to keep recording after EPG stop time.
const defaultTimeShiftAfter = 15

// defaultIntervalSec is the default watch loop sleep interval in seconds.
const defaultIntervalSec = 10

type epgEntry struct {
	title   string
	year    int
	filmID  int
	sid     int
	start   string
	stop    string
	isMovie bool
}

type channel struct {
	id   int
	key  string
	name string
}

type fwEntry struct {
	id       int
	title    string
	year     int
	ignored  bool
	recorded bool
}

type schedulableEPG struct {
	id       int
	title    string
	year     int
	fwID     int
	channel  string
	start    time.Time
	stop     time.Time
	recorder int
	pid      int
}

func (e schedulableEPG) duration() int {
	return int(e.stop.Sub(e.start).Minutes())
}

type movie struct {
	id    int
	title string
	year  int
}
