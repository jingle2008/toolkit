package logging

import (
	"errors"
	"testing"
)

type countingLogger struct {
	infos   int
	debug   bool
	syncErr error
}

func (c *countingLogger) Debugw(string, ...any)    {}
func (c *countingLogger) Infow(string, ...any)     { c.infos++ }
func (c *countingLogger) Warnw(string, ...any)     {}
func (c *countingLogger) Errorw(string, ...any)    {}
func (c *countingLogger) WithFields(...any) Logger { return c }
func (c *countingLogger) DebugEnabled() bool       { return c.debug }
func (c *countingLogger) Sync() error              { return c.syncErr }

func TestTee_ForwardsToAll(t *testing.T) {
	t.Parallel()
	a, b := &countingLogger{}, &countingLogger{}
	NewTee(a, b).Infow("hi")
	if a.infos != 1 || b.infos != 1 {
		t.Errorf("infos a=%d b=%d, want 1,1", a.infos, b.infos)
	}
}

func TestTee_WithFieldsFansOut(t *testing.T) {
	t.Parallel()
	r := NewRingSink(4)
	c := &countingLogger{}
	NewTee(c, r).WithFields("k", "v").Infow("hi")
	if c.infos != 1 {
		t.Errorf("child not called via WithFields tee")
	}
	if snap := r.Snapshot(); len(snap) != 1 || len(snap[0].Fields) != 2 {
		t.Errorf("ring did not receive fielded entry: %+v", snap)
	}
}

func TestTee_DebugEnabledIsOr(t *testing.T) {
	t.Parallel()
	if !NewTee(&countingLogger{debug: false}, &countingLogger{debug: true}).DebugEnabled() {
		t.Error("DebugEnabled should be true if any child is debug-enabled")
	}
	if NewTee(&countingLogger{}, &countingLogger{}).DebugEnabled() {
		t.Error("DebugEnabled should be false if no child is debug-enabled")
	}
}

func TestTee_SyncJoinsErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	if err := NewTee(&countingLogger{syncErr: boom}, &countingLogger{}).Sync(); !errors.Is(err, boom) {
		t.Errorf("Sync did not propagate child error: %v", err)
	}
}
