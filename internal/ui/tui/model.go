/*
Package tui implements the core TUI model and logic for the toolkit application.
It provides the Model struct and related helpers for managing state, events, and rendering
using Bubble Tea and Charmbracelet components.
*/
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
)

// loadData loads the dataset for the current model.
func (m *Model) loadData() []tea.Cmd {
	return []tea.Cmd{
		m.loadingTimer.Init(),
		m.beginTask(),
		func() tea.Msg {
			dataset, err := m.loader.LoadDataset(m.ctx, m.repoPath, m.environment)
			if err != nil {
				return ErrMsg(err)
			}
			return DataMsg{Data: dataset}
		},
	}
}

func setFilter(filter string) tea.Cmd {
	if filter == "" {
		return nil
	}

	return func() tea.Msg {
		return SetFilterMsg(filter)
	}
}

// lazyLoadedCategories is a set of categories that are loaded on demand and never mutated.
var lazyLoadedCategories = map[domain.Category]struct{}{
	domain.BaseModel:          {},
	domain.GpuPool:            {},
	domain.GpuNode:            {},
	domain.DedicatedAICluster: {},
}

// Init implements the tea.Model interface and initializes the model.
func (m *Model) Init() tea.Cmd {
	cmds := m.loadData()

	if _, ok := lazyLoadedCategories[m.category]; ok {
		cmds = append(cmds, m.updateCategory(m.category)...)
	}

	cmds = append(cmds, setFilter(m.newFilter))
	return tea.Sequence(cmds...)
}
