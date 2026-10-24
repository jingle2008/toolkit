// Package logging builds the toolkit's *slog.Logger (a rotating file
// handler, optionally fanned out to an in-memory ring for the TUI log
// overlay) and carries it through context.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Log formats accepted by --log-format.
const (
	FormatConsole = "console" // slog text handler (key=value)
	FormatJSON    = "json"    // slog JSON handler
	// FormatSlog is a deprecated alias of FormatJSON, kept so configs
	// written for the old x/exp/slog backend keep working.
	FormatSlog = "slog"
)

// ParseLevel maps a --log-level value to a slog level. An empty level
// follows the --debug flag (debug when set, info otherwise); an unknown
// one falls back to info.
func ParseLevel(level string, debug bool) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "":
		if debug {
			return slog.LevelDebug
		}
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewHandler returns a handler writing format-encoded records at or
// above level to w. Records carry their real call site (AddSource).
// FormatConsole selects the text handler; anything else (json, the
// deprecated slog alias) the JSON handler.
func NewHandler(w io.Writer, format string, level slog.Leveler) slog.Handler {
	opts := &slog.HandlerOptions{Level: level, AddSource: true}
	if format == FormatConsole {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}

// NewFile returns a logger writing to filename through lumberjack
// rotation (50 MB per file, 3 gzip'd backups, 28 days). The returned
// closer closes the current log file; call it on shutdown. level is a
// --log-level value (empty follows debug). format "slog" logs a
// deprecation warning once, at construction.
func NewFile(filename, format, level string, debug bool) (*slog.Logger, io.Closer) {
	w := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    50, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // gzip old logs
	}
	logger := slog.New(NewHandler(w, format, ParseLevel(level, debug)))
	if format == FormatSlog {
		logger.Warn("--log-format slog is deprecated and now an alias of json; use --log-format json")
	}
	return logger, w
}

// Discard returns a logger that drops every record.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

type ctxKeyLogger struct{}

// WithContext returns a copy of ctx carrying logger.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, logger)
}

// FromContext returns the logger carried by ctx, or a discard logger
// when ctx is nil or carries none.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Discard()
	}
	if logger, ok := ctx.Value(ctxKeyLogger{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return Discard()
}
