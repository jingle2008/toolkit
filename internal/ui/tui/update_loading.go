package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateLoadingView handles command routing while in LoadingView mode.
func (m *Model) updateLoadingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds, quit := m.routeLoadingMsg(msg)
	if quit {
		return m, tea.Quit
	}
	cmds = append(cmds, m.handleStopwatchMsg(msg))
	return m, tea.Batch(cmds...)
}

func (m *Model) routeLoadingMsg(msg tea.Msg) ([]tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			m.cancelInFlight()
			return nil, true
		}
		return nil, false
	case dataMsg:
		return []tea.Cmd{m.handleDataMsg(msg)}, false
	case datasetLoadedMsg:
		return []tea.Cmd{m.handleDataMsg(dataMsg{Data: msg.Dataset, Gen: msg.Gen})}, false
	case errMsg:
		if cmd := m.handleErrMsg(msg); cmd != nil {
			return []tea.Cmd{cmd}, false
		}
		return nil, false
	case spinner.TickMsg:
		return []tea.Cmd{m.handleSpinnerTickMsg(msg)}, false
	default:
		return m.routeListLoadedMsg(msg), false
	}
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
