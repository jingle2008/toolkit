// Package logging provides context-based logging utilities for the toolkit application.
package logging

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
)

// loggerKey is the context key for storing Logger.
type loggerKey struct{}

var warnOnce sync.Once

// NewLogger creates a new Logger. If debug is true, uses zap.NewDevelopment, else zap.NewProduction.
func NewLogger(debug bool) (Logger, error) {
	var zl *zap.Logger
	var err error
	if debug {
		zl, err = zap.NewDevelopment()
	} else {
		zl, err = zap.NewProduction()
	}
	if err != nil {
		return nil, err
	}
	return NewZapLogger(zl.Sugar()), nil
}

// WithLogger returns a new context with the provided Logger attached.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// LoggerFromCtx extracts a Logger from the context, or returns a no-op Logger if none is set.
// Logs a warning to stderr once if logger is missing.
func LoggerFromCtx(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerKey{}).(Logger); ok && l != nil {
		return l
	}
	warnOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "[logging] WARNING: Logger not found in context, using no-op Logger")
	})
	// Return a zapLogger wrapping zap.NewNop().Sugar()
	return NewZapLogger(zap.NewNop().Sugar())
}
