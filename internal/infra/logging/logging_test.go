package logging

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestCtxWithLoggerAndLoggerFromCtx(t *testing.T) {
	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	ctxWithLogger := CtxWithLogger(ctx, logger)
	got := LoggerFromCtx(ctxWithLogger)
	if got != logger {
		t.Errorf("LoggerFromCtx did not return the logger set by CtxWithLogger")
	}
}

func TestLoggerFromCtxReturnsNopIfNoneSet(t *testing.T) {
	ctx := context.Background()
	got := LoggerFromCtx(ctx)
	if got == nil {
		t.Errorf("LoggerFromCtx should never return nil")
	}
	// zap.NewNop returns a logger with the same pointer every time, so we can check type
	nop := zap.NewNop()
	if got.Core().Enabled(zap.DebugLevel) != nop.Core().Enabled(zap.DebugLevel) {
		t.Errorf("LoggerFromCtx should return zap.NewNop() if no logger is set")
	}
}
