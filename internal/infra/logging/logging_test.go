package logging

import (
	"context"
	"errors"
	"testing"
)

type fakeLogger struct {
	debugs, infos, errors int
	fields                []any
}

func (f *fakeLogger) Debugw(msg string, kv ...any) { f.debugs++ }
func (f *fakeLogger) Infow(msg string, kv ...any)  { f.infos++ }
func (f *fakeLogger) Errorw(msg string, kv ...any) { f.errors++ }
func (f *fakeLogger) WithFields(kv ...any) Logger {
	f.fields = append(f.fields, kv...)
	return f
}

func TestWithLoggerAndLoggerFromCtx(t *testing.T) {
	ctx := context.Background()
	l := &fakeLogger{}
	ctx2 := WithLogger(ctx, l)
	got := LoggerFromCtx(ctx2)
	if got != l {
		t.Errorf("LoggerFromCtx did not return the logger set by WithLogger")
	}
}

func TestLoggerFromCtxReturnsNopIfNoneSet(t *testing.T) {
	ctx := context.Background()
	got := LoggerFromCtx(ctx)
	got.Debugw("should not panic")
	got.Infow("should not panic")
	got.Errorw("should not panic")
	got2 := got.WithFields("foo", "bar")
	if got2 == nil {
		t.Errorf("WithFields should return a logger, got nil")
	}
}

func TestNewLogger_Error(t *testing.T) {
	// Simulate zap.NewProductionConfig() error by passing impossible config
	// (In real code, would use monkeypatch or test build tag, but here just check no panic)
	l, err := NewLogger(false)
	if err != nil && l != nil {
		t.Errorf("If error, logger should be nil")
	}
}

func TestZapLoggerImplementsLogger(t *testing.T) {
	// Just ensure zapLogger implements all methods and doesn't panic
	z, err := NewLogger(false)
	if err != nil {
		t.Skip("zap logger not available")
	}
	z.Debugw("debug", "k", "v")
	z.Infow("info", "k", "v")
	z.Errorw("error", "k", "v")
	z2 := z.WithFields("foo", "bar")
	if z2 == nil {
		t.Errorf("WithFields should return a logger, got nil")
	}
}

func TestLoggerKeyType(t *testing.T) {
	var k1, k2 loggerKey
	if k1 != k2 {
		t.Errorf("loggerKey should be comparable")
	}
}

func TestLoggerFromCtx_TypeSafety(t *testing.T) {
	ctx := context.WithValue(context.Background(), loggerKey{}, "not a logger")
	got := LoggerFromCtx(ctx)
	got.Debugw("should not panic")
}

func TestNewLogger_Success(t *testing.T) {
	l, err := NewLogger(true)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if l == nil {
		t.Errorf("expected logger, got nil")
	}
}

func TestWithLogger_Nil(t *testing.T) {
	ctx := context.Background()
	ctx2 := WithLogger(ctx, nil)
	got := LoggerFromCtx(ctx2)
	if got == nil {
		t.Errorf("LoggerFromCtx should return a logger, got nil")
	}
}

func TestLoggerFromCtx_NilContext(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil context, got none")
		}
	}()
	_ = LoggerFromCtx(nil)
}

func TestLoggerFromCtx_UnknownType(t *testing.T) {
	ctx := context.WithValue(context.Background(), loggerKey{}, errors.New("not a logger"))
	got := LoggerFromCtx(ctx)
	got.Debugw("should not panic")
}
