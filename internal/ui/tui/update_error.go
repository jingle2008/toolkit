package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

/*
handleErrMsg centralizes error handling for async operations.

It records the error, dismisses LoadingView via endTask, and surfaces
the failure as a transient toast over the restored view — so the user
can keep navigating instead of being trapped in a terminal ErrorView.
*/
func (m *Model) handleErrMsg(msg errMsg) tea.Cmd {
	m.err = msg
	m.endTask(false)
	return m.showToast(msg.Error(), toastError)
}

// updateErrorView handles command routing while in ErrorView mode.
func (m *Model) updateErrorView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(msg, keys.Quit) {
			m.cancelInFlight()
			return m, tea.Quit
		}
	}

	return m, nil
}
