// Package logging provides a generic logging interface and adapters for zap.Logger.
package logging

import (
	"go.uber.org/zap"
)

// Logger is an abstract logging interface for use throughout the codebase.
type Logger interface {
	Debugw(msg string, kv ...any)
	Infow(msg string, kv ...any)
	Errorw(msg string, kv ...any)
	WithFields(kv ...any) Logger
}

// zapLogger is an adapter that wraps a zap.SugaredLogger to implement Logger.
type zapLogger struct {
	s *zap.SugaredLogger
}

func (l *zapLogger) Debugw(msg string, kv ...any) {
	l.s.Debugw(msg, kv...)
}

func (l *zapLogger) Infow(msg string, kv ...any) {
	l.s.Infow(msg, kv...)
}

func (l *zapLogger) Errorw(msg string, kv ...any) {
	l.s.Errorw(msg, kv...)
}

func (l *zapLogger) WithFields(kv ...any) Logger {
	return &zapLogger{s: l.s.With(kv...)}
}

// NewZapLogger returns a Logger backed by a zap.SugaredLogger.
func NewZapLogger(s *zap.SugaredLogger) Logger {
	return &zapLogger{s: s}
}
