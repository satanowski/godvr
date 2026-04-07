package main

import (
	"testing"
	"time"
)

func TestSchedulableEPGDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		epg  schedulableEPG
		want int
	}{
		{
			name: "two hour movie",
			epg: schedulableEPG{
				start: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				stop:  time.Date(2026, 4, 6, 22, 0, 0, 0, time.UTC),
			},
			want: 120,
		},
		{
			name: "90 minute movie",
			epg: schedulableEPG{
				start: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				stop:  time.Date(2026, 4, 6, 21, 30, 0, 0, time.UTC),
			},
			want: 90,
		},
		{
			name: "zero duration",
			epg: schedulableEPG{
				start: time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
				stop:  time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
			},
			want: 0,
		},
		{
			name: "crosses midnight",
			epg: schedulableEPG{
				start: time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
				stop:  time.Date(2026, 4, 7, 1, 30, 0, 0, time.UTC),
			},
			want: 150,
		},
		{
			name: "short 30 minute show",
			epg: schedulableEPG{
				start: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
				stop:  time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC),
			},
			want: 30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.epg.duration()
			if got != tt.want {
				t.Errorf("duration() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	t.Run("noRecorder is negative", func(t *testing.T) {
		t.Parallel()
		if noRecorder >= 0 {
			t.Errorf("noRecorder = %d, want negative sentinel value", noRecorder)
		}
	})

	t.Run("defaultTimeShiftBefore is positive", func(t *testing.T) {
		t.Parallel()
		if defaultTimeShiftBefore <= 0 {
			t.Errorf("defaultTimeShiftBefore = %d, want positive", defaultTimeShiftBefore)
		}
	})

	t.Run("defaultTimeShiftAfter is positive", func(t *testing.T) {
		t.Parallel()
		if defaultTimeShiftAfter <= 0 {
			t.Errorf("defaultTimeShiftAfter = %d, want positive", defaultTimeShiftAfter)
		}
	})

	t.Run("defaultIntervalSec is positive", func(t *testing.T) {
		t.Parallel()
		if defaultIntervalSec <= 0 {
			t.Errorf("defaultIntervalSec = %d, want positive", defaultIntervalSec)
		}
	})

	t.Run("similarityThreshold in valid range", func(t *testing.T) {
		t.Parallel()
		if similarityThreshold <= 0 || similarityThreshold > 1 {
			t.Errorf("similarityThreshold = %f, want (0, 1]", similarityThreshold)
		}
	})
}
