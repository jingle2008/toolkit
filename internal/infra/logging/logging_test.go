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
func (f *fakeLogger) DebugEnabled() bool { return true }

func TestWithLoggerAndLoggerFromCtx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := &fakeLogger{}
	ctx2 := WithContext(ctx, l)
	got := FromContext(ctx2)
	if got != l {
		t.Errorf("FromContext did not return the logger set by WithContext")
	}
}

func TestLoggerFromCtxReturnsNopIfNoneSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := FromContext(ctx)
	got.Debugw("should not panic")
	got.Infow("should not panic")
	got.Errorw("should not panic")
	got2 := got.WithFields("foo", "bar")
	if got2 == nil {
		t.Errorf("WithFields should return a logger, got nil")
	}
}

func TestNewLogger_Error(t *testing.T) {
	t.Parallel()
	// Simulate zap.NewProductionConfig() error by passing impossible config
	// (In real code, would use monkeypatch or test build tag, but here just check no panic)
	l, err := NewLogger(false)
	if err != nil && l != nil {
		t.Errorf("If error, logger should be nil")
	}
}

func TestZapLoggerImplementsLogger(t *testing.T) {
	t.Parallel()
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

func TestNewLogger_Success(t *testing.T) {
	t.Parallel()
	l, err := NewLogger(true)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if l == nil {
		t.Errorf("expected logger, got nil")
	}
}

func TestWithContext_Nil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx2 := WithContext(ctx, nil)
	got := FromContext(ctx2)
	if got == nil {
		t.Errorf("FromContext should return a logger, got nil")
	}
}

func TestFromContext_NilContext(t *testing.T) {
	got := FromContext(context.TODO())
	got.Debugw("should not panic")
}

func TestFromContext_UnknownType(t *testing.T) {
	type unknownKeyType struct{}
	ctx := context.WithValue(context.Background(), unknownKeyType{}, errors.New("not a logger"))
	got := FromContext(ctx)
	got.Debugw("should not panic")
}

func TestNewNoOpLogger(t *testing.T) {
	t.Parallel()
	l := NewNoOpLogger()
	if l == nil {
		t.Errorf("expected non-nil logger")
	}
	if l.DebugEnabled() {
		t.Errorf("expected DebugEnabled to be false")
	}
	l.Debugw("should not panic")
	l.Infow("should not panic")
	l.Errorw("should not panic")
	l2 := l.WithFields("foo", "bar")
	if l2 == nil {
		t.Errorf("WithFields should return a logger, got nil")
	}
}

func TestMustNewLogger(t *testing.T) {
	t.Parallel()
	l := MustNewLogger(true)
	if l == nil {
		t.Errorf("expected non-nil logger")
	}
}
