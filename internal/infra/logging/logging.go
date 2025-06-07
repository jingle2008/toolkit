// Package logging provides context-based logging utilities for the toolkit application.
package logging

import (
	"context"

	"go.uber.org/zap"
)

type ctxLoggerKey struct{}

/*
CtxWithLogger returns a new context with the provided zap.Logger attached.
*/
func CtxWithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

/*
LoggerFromCtx extracts a zap.Logger from the context, or returns zap.NewNop() if none is set.
*/
func LoggerFromCtx(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return zap.NewNop()
}
