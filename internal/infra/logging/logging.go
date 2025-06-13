// Package logging provides context-based logging utilities and a generic logging interface for the toolkit application.
package logging

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is an abstract logging interface for use throughout the codebase.
type Logger interface {
	Debugw(msg string, kv ...any)
	Infow(msg string, kv ...any)
	Errorw(msg string, kv ...any)
	WithFields(kv ...any) Logger
	DebugEnabled() bool
	Sync() error
}

// zapLogger is an adapter that wraps a zap.SugaredLogger to implement Logger.
type zapLogger struct {
	s     *zap.SugaredLogger
	debug bool
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
	return &zapLogger{s: l.s.With(kv...), debug: l.debug}
}

func (l *zapLogger) DebugEnabled() bool {
	return l.debug
}

func (l *zapLogger) Sync() error {
	return l.s.Sync()
}

// NewZapLogger returns a Logger backed by a zap.SugaredLogger.
// The debug flag controls DebugEnabled().
func NewZapLogger(s *zap.SugaredLogger, debug bool) Logger {
	return &zapLogger{s: s, debug: debug}
}

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
	return NewZapLogger(zl.Sugar(), debug), nil
}

// MustNewLogger creates a new Logger or panics if creation fails.
func MustNewLogger(debug bool) Logger {
	l, err := NewLogger(debug)
	if err != nil {
		panic(err)
	}
	return l
}

// NewFileLogger returns a Logger that writes only to the given file (overwriting it on each run).
// If debug is true, uses development encoder config, else production config.
func NewFileLogger(debug bool, filename string) (Logger, error) {
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	f, err := os.OpenFile(filename, flag, 0o600) // #nosec G304
	if err != nil {
		return nil, err
	}
	var encCfg zapcore.EncoderConfig
	if debug {
		encCfg = zap.NewDevelopmentEncoderConfig()
	} else {
		encCfg = zap.NewProductionEncoderConfig()
	}
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(f),
		zap.DebugLevel,
	)
	zl := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return NewZapLogger(zl.Sugar(), debug), nil
}

// MustNewFileLogger returns a file Logger or panics if creation fails.
func MustNewFileLogger(debug bool, filename string) Logger {
	l, err := NewFileLogger(debug, filename)
	if err != nil {
		panic(err)
	}
	return l
}

// NewNoOpLogger returns a Logger that does nothing (for tests).
func NewNoOpLogger() Logger {
	return noopLogger{}
}

// ---- Context propagation ----

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

// ---- No-op logger ----

type noopLogger struct{}

func (noopLogger) Debugw(string, ...any)    {}
func (noopLogger) Infow(string, ...any)     {}
func (noopLogger) Errorw(string, ...any)    {}
func (noopLogger) WithFields(...any) Logger { return noopLogger{} }
func (noopLogger) DebugEnabled() bool       { return false }
func (noopLogger) Sync() error              { return nil }
