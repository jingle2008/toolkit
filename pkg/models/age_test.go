package models

import (
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int64
	}{
		{"1s", 1},
		{"10s", 10},
		{"2m", 120},
		{"5h", 18000},
		{"1d", 86400},
		{"0s", 0},
		{"0m", 0},
		{"0h", 0},
		{"0d", 0},
		{"", 0},
		{"s", 0},
		{"10", 0},
		{"10x", 0},
		{"-5m", 0},
		{"1000d", 86400000},
		{"999h", 3596400},
	}

	for _, tt := range tests {
		got := ParseAge(tt.input)
		if got != tt.expected {
			t.Errorf("ParseAge(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestFormatAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{time.Second * 30, "30s"},
		{time.Minute, "1m"},
		{time.Minute*59 + time.Second*59, "59m"},
		{time.Hour, "1h"},
		{time.Hour*23 + time.Minute*59, "23h"},
		{time.Hour * 48, "2d"},
		{time.Hour*72 + time.Minute*1, "3d"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			got := FormatAge(tt.dur)
			if got != tt.expected {
				t.Errorf("FormatAge(%v) = %q, want %q", tt.dur, got, tt.expected)
			}
		})
	}
}
