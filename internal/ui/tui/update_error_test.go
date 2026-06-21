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
// still runs to keep pendingTasks balanced.
func TestHandleErrMsg_CanceledLoadIsDropped(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.pendingTasks = 1

	msg := errMsg(fmt.Errorf("failed to load %s: %w", domain.GPUNode, context.Canceled))
	cmd := m.handleErrMsg(msg)

	if cmd != nil {
		t.Fatal("canceled load should not return a toast command")
	}
	if m.toast != nil {
		t.Fatalf("canceled load should not raise a toast: %+v", m.toast)
	}
	if m.pendingTasks != 0 {
		t.Fatalf("endTask must still run for a canceled load: pendingTasks=%d", m.pendingTasks)
	}
}

// A genuine load failure is surfaced as an error toast.
func TestHandleErrMsg_RealErrorShowsToast(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.pendingTasks = 1

	msg := errMsg(errors.New("boom"))
	cmd := m.handleErrMsg(msg)

	if cmd == nil {
		t.Fatal("a real error should return a toast command")
	}
	if m.toast == nil || m.toast.sev != toastError {
		t.Fatalf("a real error should raise an error toast: %+v", m.toast)
	}
	if m.pendingTasks != 0 {
		t.Fatalf("endTask must run: pendingTasks=%d", m.pendingTasks)
	}
}
