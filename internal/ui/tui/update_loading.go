package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/key"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateLoadingView handles the first-boot LoadingView (m.dataset == nil).
// Tick messages, data messages, and errors are intercepted at the top of
// Update; the only thing left to do here is honor Quit.
func (m *Model) updateLoadingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit
	}
	return m, nil
}

// handleSpinnerTickMsg advances the spinner one frame and lets the
// tick chain die when no load is in flight, so we don't burn empty
// event-loop wakeups idle. beginTask kicks off a fresh chain via
// tea.Sequence the next time pendingTasks goes 0 → 1.
func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) tea.Cmd {
	loadingSpinner, cmd := m.loadingSpinner.Update(msg)
	m.loadingSpinner = &loadingSpinner
	if m.pendingTasks == 0 {
		return nil
	}
	return cmd
}

func (m *Model) handleStopwatchMsg(msg tea.Msg) tea.Cmd {
	timer, cmd := m.loadingTimer.Update(msg)
	m.loadingTimer = &timer
	// Only the Tick chain is self-perpetuating; StartStopMsg/ResetMsg
	// are one-shot state transitions emitted by beginTask, so we keep
	// propagating those even when pendingTasks is 0.
	if _, isTick := msg.(stopwatch.TickMsg); isTick && m.pendingTasks == 0 {
		return nil
	}
	return cmd
}
