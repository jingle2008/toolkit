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
	if m.helpTyping {
		return m, m.updateHelpInput(keyMsg)
	}
	switch {
	case keyMsg.String() == "/":
		ti := textinput.New()
		ti.Prompt = "/"
		ti.SetValue(m.helpFilter)
		m.helpInput, m.helpTyping = ti, true
		return m, m.helpInput.Focus()
	case key.Matches(keyMsg, keys.Back) && m.helpFilter != "":
		m.helpFilter, m.helpScroll = "", 0
	case key.Matches(keyMsg, keys.Back, keys.Help):
		m.viewMode = m.lastViewMode
	case key.Matches(keyMsg, keys.Quit):
		m.cancelInFlight()
		return m, tea.Quit
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
		m.cancelInFlight()
		return tea.Quit
	case "enter":
		m.helpTyping = false
		return nil
	case "esc":
		m.helpTyping, m.helpFilter = false, ""
		m.helpScroll = 0
		return nil
	}
	var cmd tea.Cmd
	m.helpInput, cmd = m.helpInput.Update(km)
	m.helpFilter = strings.TrimSpace(m.helpInput.Value())
	m.helpScroll = 0
	return cmd
}
