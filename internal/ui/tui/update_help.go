package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, keys.Back, keys.Help) {
			m.viewMode = m.lastViewMode
		} else if key.Matches(keyMsg, keys.Quit) {
			return m, tea.Quit
		}
	}

	return m, nil
}
