package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestAtoi(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    string
		def  int
		want int
	}{
		{"valid positive", "42", 0, 42},
		{"valid zero", "0", 5, 0},
		{"valid negative", "-7", 0, -7},
		{"empty string returns default", "", 99, 99},
		{"non-numeric returns default", "abc", 10, 10},
		{"float returns default", "3.14", 0, 0},
		{"whitespace returns default", " 5 ", 0, 0},
		{"large number", "999999", 0, 999999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := atoi(tt.s, tt.def)
			if got != tt.want {
				t.Errorf("atoi(%q, %d) = %d, want %d", tt.s, tt.def, got, tt.want)
			}
		})
	}
}

func TestParseDateTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "valid datetime",
			input: "2026-04-06T22:30:00",
			want:  time.Date(2026, 4, 6, 22, 30, 0, 0, time.UTC),
		},
		{
			name:  "midnight",
			input: "2026-01-01T00:00:00",
			want:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "end of day",
			input: "2026-12-31T23:59:59",
			want:  time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "date only",
			input:   "2026-04-06",
			wantErr: true,
		},
		{
			name:    "wrong format",
			input:   "06/04/2026 22:30:00",
			wantErr: true,
		},
		{
			name:    "garbage",
			input:   "not-a-date",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDateTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDateTime(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDateTime(%q) unexpected error: %v", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseDateTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaxTitleLen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		epgs []schedulableEPG
		want int
	}{
		{"empty slice", nil, 0},
		{"single entry", []schedulableEPG{{title: "Hello"}}, 5},
		{
			"multiple entries",
			[]schedulableEPG{
				{title: "Short"},
				{title: "A Longer Title"},
				{title: "Medium"},
			},
			14,
		},
		{"empty titles", []schedulableEPG{{title: ""}, {title: ""}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := maxTitleLen(tt.epgs)
			if got != tt.want {
				t.Errorf("maxTitleLen() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatEPGLine(t *testing.T) {
	t.Parallel()
	epg := schedulableEPG{
		title:   "Test Movie",
		year:    2024,
		channel: "HBO",
		start:   time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		stop:    time.Date(2026, 4, 6, 22, 0, 0, 0, time.UTC),
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Width(15).Align(lipgloss.Left)

	got := formatEPGLine(epg, titleStyle)

	// The output contains styled text, so check for key substrings.
	checks := []string{"Test Movie", "2024", "2026.04.06", "20:00", "2h00", "HBO"}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("formatEPGLine() missing %q in output: %s", check, got)
		}
	}
}

func TestGetSafeMovieFileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		epg   schedulableEPG
		check func(string) bool
		desc  string
	}{
		{
			name: "simple title",
			epg:  schedulableEPG{title: "Inception", year: 2010},
			check: func(s string) bool {
				return strings.HasSuffix(s, "Inception-(2010).mts")
			},
			desc: "should end with Inception-(2010).mts",
		},
		{
			name: "title with spaces",
			epg:  schedulableEPG{title: "The Dark Knight", year: 2008},
			check: func(s string) bool {
				return strings.HasSuffix(s, "The_Dark_Knight-(2008).mts")
			},
			desc: "spaces should be replaced with underscores",
		},
		{
			name: "title with special chars",
			epg:  schedulableEPG{title: "Wall·E: A Robot's Story!", year: 2008},
			check: func(s string) bool {
				// Special chars should be replaced with underscores
				return strings.HasSuffix(s, ".mts") && strings.Contains(s, "(2008)")
			},
			desc: "should end with .mts and contain year",
		},
		{
			name: "title with allowed parens",
			epg:  schedulableEPG{title: "Movie(Extended)", year: 2020},
			check: func(s string) bool {
				return strings.Contains(s, "Movie(Extended)")
			},
			desc: "parentheses should be preserved",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getSafeMovieFileName(tt.epg)
			if !tt.check(got) {
				t.Errorf("getSafeMovieFileName() = %q, %s", got, tt.desc)
			}
		})
	}
}

func TestSafeFilenameRgx(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"alphanumeric passes through", "HelloWorld123", "HelloWorld123"},
		{"spaces replaced", "Hello World", "Hello_World"},
		{"special chars replaced", "Hello@World#!", "Hello_World__"},
		{"dots preserved", "file.name", "file.name"},
		{"dashes preserved", "file-name", "file-name"},
		{"underscores preserved", "file_name", "file_name"},
		{"parens preserved", "Movie(2024)", "Movie(2024)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := safeFilenameRgx.ReplaceAllString(tt.input, "_")
			if got != tt.want {
				t.Errorf("safeFilenameRgx(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
