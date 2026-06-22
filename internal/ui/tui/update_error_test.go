package tui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
)

// A canceled in-flight load (the expected result of navigating away or
// quitting) must be dropped quietly: no error toast, no command. endTask
// still runs to keep pendingTasks balanced. Uses the current gen so it
// exercises the cancellation branch, not the stale-gen branch.
func TestHandleErrMsg_CanceledLoadIsDropped(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.pendingTasks = 1

	msg := errMsg{err: fmt.Errorf("failed to load %s: %w", domain.GPUNode, context.Canceled), Gen: 2}
	cmd := m.handleErrMsg(msg)

	if cmd != nil {
		t.Fatal("canceled load should not return a toast command")
	}
	if m.toasts.active != nil {
		t.Fatalf("canceled load should not raise a toast: %+v", m.toasts.active)
	}
	if m.pendingTasks != 0 {
		t.Fatalf("endTask must still run for a canceled load: pendingTasks=%d", m.pendingTasks)
	}
}

// An error from a superseded load (gen advanced because the user navigated on)
// must be dropped: no toast, no command, but endTask still runs.
func TestHandleErrMsg_StaleGenIsDropped(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 5
	m.pendingTasks = 1

	msg := errMsg{err: errors.New("boom"), Gen: 3} // from an older load
	cmd := m.handleErrMsg(msg)

	if cmd != nil {
		t.Fatal("stale-gen error should not return a toast command")
	}
	if m.toasts.active != nil {
		t.Fatalf("stale-gen error should not raise a toast: %+v", m.toasts.active)
	}
	if m.pendingTasks != 0 {
		t.Fatalf("endTask must still run for a stale error: pendingTasks=%d", m.pendingTasks)
	}
}

// A genuine, current-generation load failure is surfaced as an error toast.
func TestHandleErrMsg_RealErrorShowsToast(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.pendingTasks = 1

	msg := errMsg{err: errors.New("boom"), Gen: 2}
	cmd := m.handleErrMsg(msg)

	if cmd == nil {
		t.Fatal("a real error should return a toast command")
	}
	if m.toasts.active == nil || m.toasts.active.sev != toastError {
		t.Fatalf("a real error should raise an error toast: %+v", m.toasts.active)
	}
	if m.pendingTasks != 0 {
		t.Fatalf("endTask must run: pendingTasks=%d", m.pendingTasks)
	}
}

// The Gen-0 sentinel (the foundational Init load) is never stale-dropped, even
// when the model's generation has advanced.
func TestHandleErrMsg_Gen0SentinelAlwaysShows(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 7
	m.pendingTasks = 1

	msg := errMsg{err: errors.New("init failed")} // Gen 0
	cmd := m.handleErrMsg(msg)

	if cmd == nil || m.toasts.active == nil {
		t.Fatalf("gen-0 error must surface a toast even at gen %d", m.gen)
	}
}
