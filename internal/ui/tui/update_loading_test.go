package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeErrMsg string

func (e fakeErrMsg) Error() string { return string(e) }

func TestUpdateLoadingView_QuitKey(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
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
	msg := ErrMsg(fakeErrMsg("fail"))
	_, cmd := m.updateLoadingView(msg)
	if m.err == nil || m.err.Error() != "fail" {
		t.Errorf("expected err to be set to 'fail', got %v", m.err)
	}
	if cmd != nil {
		t.Errorf("expected nil command for ErrMsg, got %v", cmd)
	}
}

func TestUpdateLoadingView_SpinnerTickMsg(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.loadingSpinner = &spinner.Model{}
	msg := spinner.TickMsg{}
	_, cmd := m.updateLoadingView(msg)
	if cmd == nil {
		t.Error("expected non-nil command for spinner.TickMsg")
	}
}
