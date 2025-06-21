// Package toolkit: update_detail.go
// Contains updateDetailView and related detail view logic split from model_update.go.

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Back, keys.ViewDetails):
			m.exitDetailView()
		case key.Matches(keyMsg, keys.CopyName):
			m.copyItemName(findItem(m.dataset, m.category, m.choice))
		case key.Matches(keyMsg, keys.Help):
			m.enterHelpView()
		}
	}

	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = &updatedViewport
	return m, cmd
}
