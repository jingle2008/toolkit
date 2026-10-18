package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateHelpView scrolls help (j/k, pgup/pgdn, home/end, gg/G) and closes
// it on esc or the help key. A pending g takes the next key first, so esc
// then cancels just the prefix.
func (m *Model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, keys.Quit) && m.prefix.key == "" {
		m.cancelInFlight()
		return m, tea.Quit
	}
	if seq, used, cmd := m.feedPrefix(keyMsg.String(), gotoLeader); used {
		if seq == gotoLeader+" "+gotoLeader {
			m.scrollHelp("home")
		}
		return m, cmd
	}
	if key.Matches(keyMsg, keys.Back, keys.Help) {
		m.viewMode = m.lastViewMode
		return m, nil
	}
	m.scrollHelp(keyMsg.String())
	return m, nil
}
