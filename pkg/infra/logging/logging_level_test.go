package logging

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/slog"
)

func TestParseZapLevel_Mapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		level     string
		debugFlag bool
		want      zapcore.Level
	}{
		{"debug", "debug", false, zapcore.DebugLevel},
		{"info", "info", false, zapcore.InfoLevel},
		{"warn", "warn", false, zapcore.WarnLevel},
		{"warning", "warning", false, zapcore.WarnLevel},
		{"error", "error", false, zapcore.ErrorLevel},
		{"empty_debug_true", "", true, zapcore.DebugLevel},
		{"empty_debug_false", "", false, zapcore.InfoLevel},
		{"unknown_defaults_info", "not-a-level", false, zapcore.InfoLevel},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseZapLevel(tc.level, tc.debugFlag)
			if got != tc.want {
				t.Fatalf("parseZapLevel(%q, debug=%v) = %v, want %v", tc.level, tc.debugFlag, got, tc.want)
			}
		})
	}
}

func TestParseSlogLevel_Mapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		level     string
		debugFlag bool
		want      slog.Level
	}{
		{"debug", "debug", false, slog.LevelDebug},
		{"info", "info", false, slog.LevelInfo},
		{"warn", "warn", false, slog.LevelWarn},
		{"warning", "warning", false, slog.LevelWarn},
		{"error", "error", false, slog.LevelError},
		{"empty_debug_true", "", true, slog.LevelDebug},
		{"empty_debug_false", "", false, slog.LevelInfo},
		{"unknown_defaults_info", "not-a-level", false, slog.LevelInfo},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseSlogLevel(tc.level, tc.debugFlag)
			if got != tc.want {
				t.Fatalf("parseSlogLevel(%q, debug=%v) = %v, want %v", tc.level, tc.debugFlag, got, tc.want)
			}
		})
	}
}

func TestNewFileLoggerWithLevel_NoError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "level_test.log")

	// Should not error for valid combinations and should Sync successfully.
	l, err := NewFileLoggerWithLevel(false, logPath, "json", "error")
	if err != nil {
		t.Fatalf("NewFileLoggerWithLevel returned error: %v", err)
	}
	l.Infow("this should not be visible at level=error, but call should not fail")
	if err := l.Sync(); err != nil {
		t.Fatalf("logger Sync returned error: %v", err)
	}
}
