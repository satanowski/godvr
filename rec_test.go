package main

import (
	"os"
	"testing"
)

func TestValidChannelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple name", "HBO", true},
		{"name with space", "Canal Plus", true},
		{"name with dash", "HBO-HD", true},
		{"name with underscore", "canal_plus", true},
		{"name with dot", "TVP1.HD", true},
		{"name with digits", "TVP2", true},
		{"empty string", "", false},
		{"semicolon injection", "HBO;rm -rf /", false},
		{"pipe injection", "HBO|cat /etc/passwd", false},
		{"backtick injection", "HBO`whoami`", false},
		{"dollar injection", "HBO$HOME", false},
		{"newline injection", "HBO\nid", false},
		{"ampersand", "HBO&", false},
		{"parentheses", "HBO()", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validChannelName.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("validChannelName.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecorderProcessExists(t *testing.T) {
	t.Parallel()

	t.Run("current process exists", func(t *testing.T) {
		t.Parallel()
		// Our own PID should exist.
		if !recorderProcessExists(os.Getpid()) {
			t.Error("recorderProcessExists(own pid) = false, want true")
		}
	})

	t.Run("nonexistent pid", func(t *testing.T) {
		t.Parallel()
		// PID 0 sends signal to every process in the group; use a very unlikely PID instead.
		// A very high PID is unlikely to exist.
		if recorderProcessExists(4194304) {
			t.Error("recorderProcessExists(4194304) = true, want false")
		}
	})
}
