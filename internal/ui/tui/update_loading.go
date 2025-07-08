package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateLoadingView handles command routing while in LoadingView mode.
func (m *Model) updateLoadingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
	case DataMsg:
		m.handleDataMsg(msg)
	case ErrMsg:
		m.handleErrMsg(msg)
	case spinner.TickMsg:
		return m, m.handleSpinnerTickMsg(msg)
	}
	return m, nil
}

func (m *Model) handleErrMsg(msg ErrMsg) {
	m.err = msg
	m.endTask(false)
}

func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) tea.Cmd {
	loadingSpinner, cmd := m.loadingSpinner.Update(msg)
	m.loadingSpinner = &loadingSpinner
	return cmd
}
