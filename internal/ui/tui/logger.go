package tui

import "go.uber.org/zap"

// Logger is a minimal logging interface for decoupling from zap.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}

// zapLogger is an adapter for zap.Logger to implement Logger.
type zapLogger struct {
	z *zap.Logger
}

func (l *zapLogger) Debug(msg string, fields ...zap.Field) { l.z.Debug(msg, fields...) }
func (l *zapLogger) Info(msg string, fields ...zap.Field)  { l.z.Info(msg, fields...) }
func (l *zapLogger) Warn(msg string, fields ...zap.Field)  { l.z.Warn(msg, fields...) }
func (l *zapLogger) Error(msg string, fields ...zap.Field) { l.z.Error(msg, fields...) }

// NewZapLogger returns a Logger backed by a zap.Logger.
func NewZapLogger(z *zap.Logger) Logger {
	return &zapLogger{z: z}
}
