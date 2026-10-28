package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateHelpView handles keys while help is open. / opens the filter,
// which narrows help as it is typed (enter keeps it, esc clears it); with
// a filter set, esc clears it before esc (or ?/h) closes help; the rest
// scrolls.
func (m *Model) updateHelpView(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.help.typing {
		return m, m.updateHelpInput(keyMsg)
	}
	switch {
	case keyMsg.String() == "/":
		ti := textinput.New()
		ti.Prompt = "/"
		ti.SetValue(m.help.filter)
		m.help.input, m.help.typing = ti, true
		return m, m.help.input.Focus()
	case key.Matches(keyMsg, keys.Back) && m.help.filter != "":
		m.help.filter, m.help.scroll = "", 0
	case key.Matches(keyMsg, keys.Back, keys.Help):
		m.closeView()
	case key.Matches(keyMsg, keys.Quit):
		return m, m.quit()
	default:
		m.scrollHelp(keyMsg.String())
	}
	return m, nil
}

// updateHelpInput feeds km to the help filter, narrowing help as it is
// typed.
func (m *Model) updateHelpInput(km tea.KeyPressMsg) tea.Cmd {
	switch km.String() {
	case "ctrl+c":
		return m.quit()
	case "enter":
		m.help.typing = false
		return nil
	case "esc":
		m.help.typing, m.help.filter = false, ""
		m.help.scroll = 0
		return nil
	}
	var cmd tea.Cmd
	m.help.input, cmd = m.help.input.Update(km)
	m.help.filter = strings.TrimSpace(m.help.input.Value())
	m.help.scroll = 0
	return cmd
}
