package tui

import (
	"strings"
	"testing"
)

func TestShowToast_SetsStateAndReturnsCmd(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.showToast("boom", toastError)
	if m.toasts.active == nil {
		t.Fatal("expected toast to be set")
	}
	if m.toasts.active.msg != "boom" || m.toasts.active.sev != toastError {
		t.Errorf("unexpected toast state: %+v", m.toasts.active)
	}
	if m.toasts.active.id == 0 {
		t.Error("expected non-zero toast id")
	}
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd for auto-dismiss tick")
	}
}

func TestShowToast_MonotonicIDs(t *testing.T) {
	t.Parallel()
	m := &Model{}
	_ = m.showToast("first", toastError)
	first := m.toasts.active.id
	_ = m.showToast("second", toastInfo)
	if m.toasts.active.id <= first {
		t.Errorf("expected newer toast to have a higher id, got %d after %d", m.toasts.active.id, first)
	}
}

func TestHandleToastExpireMsg_ClearsMatching(t *testing.T) {
	t.Parallel()
	m := &Model{}
	_ = m.showToast("boom", toastError)
	id := m.toasts.active.id
	m.handleToastExpireMsg(toastExpireMsg{id: id})
	if m.toasts.active != nil {
		t.Errorf("expected toast cleared, still got %+v", m.toasts.active)
	}
}

func TestHandleToastExpireMsg_IgnoresStaleID(t *testing.T) {
	t.Parallel()
	m := &Model{}
	_ = m.showToast("first", toastError)
	stale := m.toasts.active.id
	_ = m.showToast("second", toastError) // bumps seq, replaces toast
	m.handleToastExpireMsg(toastExpireMsg{id: stale})
	if m.toasts.active == nil {
		t.Fatal("expected newer toast to survive a stale expiry")
	}
	if m.toasts.active.msg != "second" {
		t.Errorf("unexpected toast after stale expiry: %+v", m.toasts.active)
	}
}

func TestRenderToast_NilReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := &Model{}
	if got := m.renderToast(40); got != "" {
		t.Errorf("expected empty render for nil toast, got %q", got)
	}
}

func TestRenderToast_ContainsMessage(t *testing.T) {
	t.Parallel()
	m := &Model{}
	_ = m.showToast("kubectl: connection refused", toastError)
	got := m.renderToast(60)
	if !strings.Contains(got, "kubectl") {
		t.Errorf("expected rendered toast to contain message text, got %q", got)
	}
}
