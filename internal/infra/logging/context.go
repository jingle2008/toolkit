// Package logging provides context-based logger propagation utilities.
package logging

import (
	"context"
)

type ctxKeyLogger struct{}

// WithContext returns a new context with the provided Logger attached.
func WithContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, logger)
}

// FromContext retrieves the Logger from the context, or returns a no-op logger if not found.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return noopLogger{}
	}
	if logger, ok := ctx.Value(ctxKeyLogger{}).(Logger); ok && logger != nil {
		return logger
	}
	return noopLogger{}
}

// noopLogger implements Logger but does nothing.
type noopLogger struct{}

func (noopLogger) Debugw(string, ...any)    {}
func (noopLogger) Infow(string, ...any)     {}
func (noopLogger) Errorw(string, ...any)    {}
func (noopLogger) WithFields(...any) Logger { return noopLogger{} }
