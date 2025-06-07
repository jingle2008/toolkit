package logging

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestCtxWithLoggerAndLoggerFromCtx(t *testing.T) {
	ctx := context.Background()
	zapLogger := zap.NewNop().Sugar()
	logger := NewZapLogger(zapLogger)
	ctxWithLogger := WithLogger(ctx, logger)
	got := LoggerFromCtx(ctxWithLogger)
	if got == nil {
		t.Errorf("LoggerFromCtx did not return a logger")
	}
}

func TestLoggerFromCtxReturnsNopIfNoneSet(t *testing.T) {
	ctx := context.Background()
	got := LoggerFromCtx(ctx)
	if got == nil {
		t.Errorf("LoggerFromCtx should never return nil")
	}
	// Should not panic or error when calling Infow
	got.Infow("test log", "key", "value")
}
