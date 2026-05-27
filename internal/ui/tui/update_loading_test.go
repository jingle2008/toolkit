package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeErrMsg string

func (e fakeErrMsg) Error() string { return string(e) }

func TestUpdateLoadingView_QuitKey(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.updateLoadingView(msg)
	if cmd == nil {
		t.Fatal("expected tea.Quit command, got nil")
	}
}

func TestUpdateLoadingView_ErrMsg(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	m.logger = &fakeLogger{}
	msg := errMsg(fakeErrMsg("fail"))
	_, cmd := m.updateLoadingView(msg)
	if m.toast == nil || m.toast.msg != "fail" || m.toast.sev != toastError {
		t.Errorf("expected error toast with msg 'fail', got %+v", m.toast)
	}
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd (toast auto-dismiss tick), got nil")
	}
}

func TestUpdateLoadingView_SpinnerTickMsg(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	msg := spinner.TickMsg{}
	_, cmd := m.updateLoadingView(msg)
	if cmd == nil {
		t.Error("expected non-nil command for spinner.TickMsg")
	}
}

func TestUpdateLoadingView_StopwatchTickMsg(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	// The stopwatch.TickMsg handler always returns nil (no-op) for a zero stopwatch,
	// so we only check that it does not panic.
	msg := stopwatch.TickMsg{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic in handleStopwatchTickMsg: %v", r)
		}
	}()
	_, _ = m.updateLoadingView(msg)
}
