package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

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
