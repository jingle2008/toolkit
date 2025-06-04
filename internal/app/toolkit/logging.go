package toolkit

import (
	"context"

	"go.uber.org/zap"
)

type ctxLoggerKey struct{}

func CtxWithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

func LoggerFromCtx(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return zap.NewNop()
}
