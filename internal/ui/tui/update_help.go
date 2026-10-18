package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Back, keys.Help):
			m.viewMode = m.lastViewMode
		case key.Matches(keyMsg, keys.Quit):
			m.cancelInFlight()
			return m, tea.Quit
		default:
			m.scrollHelp(keyMsg.String())
		}
	}

	return m, nil
}
