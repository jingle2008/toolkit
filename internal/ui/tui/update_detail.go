// Package toolkit: update_detail.go
// Contains updateDetailView and related detail view logic split from model_update.go.

package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) updateDetailView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" {
			m.exitDetailView()
		}
	}

	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = &updatedViewport
	return m, cmd
}
