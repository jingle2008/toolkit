package logging

import (
	"sync"
	"testing"
)

func TestRingSink_AppendAndSnapshot(t *testing.T) {
	t.Parallel()
	r := NewRingSink(10)
	r.Infow("hello", "k", "v")
	r.Warnw("watch out")
	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("got %d entries, want 2", len(snap))
	}
	if snap[0].Message != "hello" || snap[0].Level != LevelInfo {
		t.Errorf("entry 0 = %+v", snap[0])
	}
	if snap[1].Level != LevelWarn {
		t.Errorf("entry 1 level = %v, want Warn", snap[1].Level)
	}
}

func TestRingSink_WrapsOnOverflow(t *testing.T) {
	t.Parallel()
	r := NewRingSink(3)
	for i := 0; i < 5; i++ {
		r.Debugw("m")
	}
	snap := r.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("got %d, want 3 (capacity)", len(snap))
	}
}

func TestRingSink_SnapshotIsCopy(t *testing.T) {
	t.Parallel()
	r := NewRingSink(4)
	r.Infow("a")
	snap := r.Snapshot()
	snap[0].Message = "mutated"
	if got := r.Snapshot(); got[0].Message != "a" {
		t.Errorf("snapshot mutation leaked into ring: %q", got[0].Message)
	}
}

func TestRingSink_WithFieldsPrepends(t *testing.T) {
	t.Parallel()
	r := NewRingSink(4)
	r.WithFields("req", "1").Infow("hi", "k", "v")
	snap := r.Snapshot()
	if len(snap) != 1 || len(snap[0].Fields) != 4 {
		t.Fatalf("fields = %+v", snap[0].Fields)
	}
	if snap[0].Fields[0] != "req" || snap[0].Fields[2] != "k" {
		t.Errorf("field order wrong: %+v", snap[0].Fields)
	}
}

func TestRingSink_DebugEnabledAlwaysTrue(t *testing.T) {
	t.Parallel()
	if !NewRingSink(1).DebugEnabled() {
		t.Error("RingSink.DebugEnabled() should be true")
	}
}

func TestRingSink_ConcurrentWrites(t *testing.T) { //nolint:paralleltest // intentionally runs concurrent writers to test synchronization
	r := NewRingSink(1000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.Infow("x") }()
	}
	wg.Wait()
	if len(r.Snapshot()) != 50 {
		t.Errorf("got %d, want 50", len(r.Snapshot()))
	}
}
