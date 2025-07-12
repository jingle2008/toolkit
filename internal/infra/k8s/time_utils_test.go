package k8s

import "testing"

func TestParseAge(t *testing.T) {
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
