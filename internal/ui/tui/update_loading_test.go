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

// errMsg, spinner.TickMsg, stopwatch.TickMsg are now routed at the top
// of Update rather than via updateLoadingView. Tests exercise that path.

func TestUpdate_ErrMsgRoutesToToast(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	m.logger = &fakeLogger{}
	_, cmd := m.Update(errMsg{err: fakeErrMsg("fail")})
	if m.toast == nil || m.toast.msg != "fail" || m.toast.sev != toastError {
		t.Errorf("expected error toast with msg 'fail', got %+v", m.toast)
	}
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd (toast auto-dismiss tick), got nil")
	}
}

func TestUpdate_SpinnerTickRoutesAtTop(t *testing.T) {
	t.Parallel()
	m := &Model{pendingTasks: 1} // a load is in flight
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	_, cmd := m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Error("expected non-nil command for spinner.TickMsg while a load is pending")
	}
}

func TestUpdate_StopwatchTickRoutesAtTop(t *testing.T) {
	t.Parallel()
	m := &Model{pendingTasks: 1}
	m.loadingSpinner = &spinner.Model{}
	m.loadingTimer = &stopwatch.Model{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic in stopwatch routing: %v", r)
		}
	}()
	_, _ = m.Update(stopwatch.TickMsg{})
}

// TestTickGate pins issue #5 from code review: when pendingTasks
// drops to 0 the spinner/stopwatch tick chain must die instead of
// self-perpetuating forever. beginTask re-kicks them on the next load.
func TestTickGate_SpinnerStopsWhenIdle(t *testing.T) {
	t.Parallel()
	m := &Model{pendingTasks: 0}
	m.loadingSpinner = &spinner.Model{}
	cmd := m.handleSpinnerTickMsg(spinner.TickMsg{})
	if cmd != nil {
		t.Errorf("expected nil cmd when pendingTasks=0 (tick chain should die), got %v", cmd)
	}
}

func TestTickGate_StopwatchStopsWhenIdle(t *testing.T) {
	t.Parallel()
	m := &Model{pendingTasks: 0}
	m.loadingTimer = &stopwatch.Model{}
	cmd := m.handleStopwatchMsg(stopwatch.TickMsg{})
	if cmd != nil {
		t.Errorf("expected nil cmd when pendingTasks=0 (tick chain should die), got %v", cmd)
	}
}
