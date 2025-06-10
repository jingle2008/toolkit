/*
Package tui implements the core TUI model and logic for the toolkit application.
It provides the Model struct and related helpers for managing state, events, and rendering
using Bubble Tea and Charmbracelet components.
*/
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// loadData loads the dataset for the current model.
func (m *Model) loadData(ctx context.Context) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		dataset, err := m.loader.LoadDataset(ctx, m.repoPath, m.environment)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return DataMsg{Data: dataset}
	}
}

// Init implements the tea.Model interface and initializes the model.
func (m *Model) Init() tea.Cmd {
	return tea.Sequence(
		m.loadingSpinner.Tick,
		m.loadData(context.Background()),
	)
}
