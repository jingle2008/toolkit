package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
)

func (m *Model) copyItemNameCmd(item any) tea.Cmd {
	return func() tea.Msg {
		actions.CopyItemName(item, m.environment, m.logger)
		return nil
	}
}
