package logging

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/slog"
)

type levelCase struct {
	name      string
	level     string
	debugFlag bool
	wantZap   zapcore.Level
	wantSlog  slog.Level
}

func levelCases() []levelCase {
	return []levelCase{
		{"debug", "debug", false, zapcore.DebugLevel, slog.LevelDebug},
		{"info", "info", false, zapcore.InfoLevel, slog.LevelInfo},
		{"warn", "warn", false, zapcore.WarnLevel, slog.LevelWarn},
		{"warning", "warning", false, zapcore.WarnLevel, slog.LevelWarn},
		{"error", "error", false, zapcore.ErrorLevel, slog.LevelError},
		{"empty_debug_true", "", true, zapcore.DebugLevel, slog.LevelDebug},
		{"empty_debug_false", "", false, zapcore.InfoLevel, slog.LevelInfo},
		{"unknown_defaults_info", "not-a-level", false, zapcore.InfoLevel, slog.LevelInfo},
	}
}

func TestParseZapLevel_Mapping(t *testing.T) {
	t.Parallel()
	for _, tc := range levelCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseZapLevel(tc.level, tc.debugFlag)
			if got != tc.wantZap {
				t.Fatalf("parseZapLevel(%q, debug=%v) = %v, want %v", tc.level, tc.debugFlag, got, tc.wantZap)
			}
		})
	}
}

func TestParseSlogLevel_Mapping(t *testing.T) {
	t.Parallel()
	for _, tc := range levelCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseSlogLevel(tc.level, tc.debugFlag)
			if got != tc.wantSlog {
				t.Fatalf("parseSlogLevel(%q, debug=%v) = %v, want %v", tc.level, tc.debugFlag, got, tc.wantSlog)
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
