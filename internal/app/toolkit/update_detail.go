// Package toolkit: update_detail.go
// Contains updateDetailView and related detail view logic split from model_update.go.

package toolkit

import tea "github.com/charmbracelet/bubbletea"

func updateDetailView(msg tea.Msg, m *Model) (tea.Model, tea.Cmd) {
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
