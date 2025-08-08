// Package toolkit: update_detail.go
// Contains updateDetailView and related detail view logic split from model_update.go.

package tui

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, keys.Back, keys.ViewDetails):
			m.exitDetailView()
		case key.Matches(keyMsg, keys.CopyName):
			actions.CopyItemName(findItem(m.dataset, m.category, m.choice), m.environment, m.logger)
		case key.Matches(keyMsg, keys.Help):
			m.enterHelpView()
		case key.Matches(keyMsg, keys.CopyObject):
			m.copyItemJSON(findItem(m.dataset, m.category, m.choice))
		}
	}

	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = &updatedViewport
	return m, cmd
}

func (m *Model) copyItemJSON(item any) {
	content, err := jsonutil.PrettyJSON(item)
	if err != nil {
		m.logger.Errorw("failed to convert item to JSON", "error", err)
		return
	}
	if err := clipboard.WriteAll(content); err != nil {
		m.logger.Errorw("failed to copy JSON to clipboard", "error", err)
	}
}
