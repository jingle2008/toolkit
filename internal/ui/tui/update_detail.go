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
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Quit):
			m.cancelInFlight()
			return m, tea.Quit
		case key.Matches(keyMsg, keys.Back, keys.ViewDetails):
			m.exitDetailView()
		case key.Matches(keyMsg, keys.CopyName):
			cmds = append(cmds, m.copyItemNameByChoice())
		case key.Matches(keyMsg, keys.Help):
			m.enterHelpView()
		case key.Matches(keyMsg, keys.CopyObject):
			cmds = append(cmds, m.copyItemJSONByChoice())
		}
	}

	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = &updatedViewport
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) copyItemNameByChoice() tea.Cmd {
	item := findItem(m.dataset, m.category, m.choice)
	return func() tea.Msg {
		actions.CopyItemName(item, m.environment, m.logger)
		return nil
	}
}

func (m *Model) copyItemJSONByChoice() tea.Cmd {
	item := findItem(m.dataset, m.category, m.choice)
	return m.copyItemJSON(item)
}

func (m *Model) copyItemJSON(item any) tea.Cmd {
	return func() tea.Msg {
		content, err := jsonutil.PrettyJSON(item)
		if err != nil {
			m.logger.Errorw("failed to convert item to JSON", "error", err)
			return nil
		}
		if err := clipboard.WriteAll(content); err != nil {
			m.logger.Errorw("failed to copy JSON to clipboard", "error", err)
		}
		return nil
	}
}
