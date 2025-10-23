package utils

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "less than a second",
			duration: 500 * time.Millisecond,
			want:     "0s",
		},
		{
			name:     "exactly one second",
			duration: time.Second,
			want:     "1s",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "exactly one minute",
			duration: time.Minute,
			want:     "1m",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 34*time.Second,
			want:     "2m 34s",
		},
		{
			name:     "minutes without seconds",
			duration: 5 * time.Minute,
			want:     "5m",
		},
		{
			name:     "exactly one hour",
			duration: time.Hour,
			want:     "1h",
		},
		{
			name:     "hours, minutes, and seconds",
			duration: 3*time.Hour + 2*time.Minute + 15*time.Second,
			want:     "3h 2m 15s",
		},
		{
			name:     "hours and seconds (no minutes)",
			duration: 2*time.Hour + 30*time.Second,
			want:     "2h 30s",
		},
		{
			name:     "hours only",
			duration: 5 * time.Hour,
			want:     "5h",
		},
		{
			name:     "boundary - 59 seconds",
			duration: 59 * time.Second,
			want:     "59s",
		},
		{
			name:     "boundary - 60 seconds (1 minute)",
			duration: 60 * time.Second,
			want:     "1m",
		},
		{
			name:     "boundary - 59 minutes 59 seconds",
			duration: 59*time.Minute + 59*time.Second,
			want:     "59m 59s",
		},
		{
			name:     "boundary - 60 minutes (1 hour)",
			duration: 60 * time.Minute,
			want:     "1h",
		},
		{
			name:     "large duration - 24 hours",
			duration: 24 * time.Hour,
			want:     "24h",
		},
		{
			name:     "rounding - 1.4 seconds",
			duration: 1400 * time.Millisecond,
			want:     "1s",
		},
		{
			name:     "rounding - 1.5 seconds",
			duration: 1500 * time.Millisecond,
			want:     "2s",
		},
		{
			name:     "rounding - 1.6 seconds",
			duration: 1600 * time.Millisecond,
			want:     "2s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncate with ellipsis",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "maxLen is 3",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen is 2",
			input:  "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen is 1",
			input:  "hello",
			maxLen: 1,
			want:   "h",
		},
		{
			name:   "maxLen is 0",
			input:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 5,
			want:   "",
		},
		{
			name:   "empty string with maxLen 0",
			input:  "",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "truncate exactly at ellipsis boundary",
			input:  "hello",
			maxLen: 4,
			want:   "h...",
		},
		{
			name:   "long string truncated",
			input:  "this is a very long string that needs truncation",
			maxLen: 20,
			want:   "this is a very lo...",
		},
		{
			name:   "unicode characters",
			input:  "hello 世界",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "single character with maxLen 1",
			input:  "a",
			maxLen: 1,
			want:   "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
