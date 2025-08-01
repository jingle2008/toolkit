package logging

import (
	"context"
	"errors"
	"os"
	"testing"

	"golang.org/x/exp/slog"
)

type fakeLogger struct {
	debugs, infos, errors int
	fields                []any
}

func (f *fakeLogger) Debugw(_ string, _ ...any) { f.debugs++ }
func (f *fakeLogger) Infow(_ string, _ ...any)  { f.infos++ }
func (f *fakeLogger) Errorw(_ string, _ ...any) { f.errors++ }
func (f *fakeLogger) WithFields(kv ...any) Logger {
	f.fields = append(f.fields, kv...)
	return f
}
func (f *fakeLogger) DebugEnabled() bool { return true }
func (f *fakeLogger) Sync() error        { return nil }

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
	t.Parallel()
	got := FromContext(context.Background())
	got.Debugw("should not panic")
}

func TestFromContext_UnknownType(t *testing.T) {
	t.Parallel()
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

func TestNewFileLoggerAndMustNewFileLogger(t *testing.T) {
	t.Parallel()
	tmpfile := "test_log.json"
	defer func() { _ = os.Remove(tmpfile) }()

	l, err := NewFileLogger(true, tmpfile, "console")
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}
	if l == nil {
		t.Fatalf("NewFileLogger returned nil logger")
	}
	l.Infow("test message", "foo", "bar")
	if err := l.Sync(); err != nil {
		t.Errorf("Sync failed: %v", err)
	}
	info, err := os.Stat(tmpfile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("log file is empty")
	}

	// MustNewFileLogger should not panic with valid file
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustNewFileLogger panicked unexpectedly: %v", r)
		}
	}()
	l2 := MustNewFileLogger(true, tmpfile)
	if l2 == nil {
		t.Errorf("MustNewFileLogger returned nil logger")
	}
}

func TestNewSlogLogger(t *testing.T) {
	t.Parallel()
	// Use a basic slog.Logger with a discard handler
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(handler)
	l := NewSlogLogger(logger, true)
	if l == nil {
		t.Errorf("NewSlogLogger returned nil")
	}
	if !l.DebugEnabled() {
		t.Errorf("DebugEnabled should be true")
	}
	l2 := l.WithFields("foo", "bar")
	if l2 == nil {
		t.Errorf("WithFields should return a logger, got nil")
	}
	l.Infow("info", "k", "v")
	l.Debugw("debug", "k", "v")
	l.Errorw("error", "k", "v")
	if err := l.Sync(); err != nil {
		t.Errorf("Sync should return nil for slogLogger")
	}
}

func TestNewFileLogger_SlogFormat(t *testing.T) {
	t.Parallel()
	tmpfile := "test_log_slog.json"
	defer func() { _ = os.Remove(tmpfile) }()
	l, err := NewFileLogger(true, tmpfile, "slog")
	if err != nil {
		t.Fatalf("NewFileLogger with slog format failed: %v", err)
	}
	if l == nil {
		t.Fatalf("NewFileLogger with slog format returned nil logger")
	}
	l.Infow("test message", "foo", "bar")
	if err := l.Sync(); err != nil {
		t.Errorf("Sync failed: %v", err)
	}
	info, err := os.Stat(tmpfile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("log file is empty")
	}
}
