package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

/*
handleErrMsg centralizes error handling for async operations.

It stores the error on the model and transitions the UI out of LoadingView
by calling endTask(false), which sets the view to ErrorView and logs timing.
*/
func (m *Model) handleErrMsg(msg ErrMsg) {
	m.err = msg
	m.endTask(false)
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
