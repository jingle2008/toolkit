package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
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

func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) tea.Cmd {
	loadingSpinner, cmd := m.loadingSpinner.Update(msg)
	m.loadingSpinner = &loadingSpinner
	return cmd
}

func (m *Model) handleStopwatchMsg(msg tea.Msg) tea.Cmd {
	timer, cmd := m.loadingTimer.Update(msg)
	m.loadingTimer = &timer
	return cmd
}
