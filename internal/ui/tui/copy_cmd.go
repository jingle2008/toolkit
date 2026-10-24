package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
)

func (m *Model) copyItemNameCmd(item any) tea.Cmd {
	env, logger, item := m.environment, m.logger, snapshotItem(item)
	return func() tea.Msg {
		actions.CopyItemName(item, env, logger)
		return nil
	}
}
