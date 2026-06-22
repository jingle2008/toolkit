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

// bumpGen increments the message generation counter and returns the new value.
func (m *Model) bumpGen() int {
	return m.gens.nextMsg()
}

// loadData loads the dataset for the current model.
func (m *Model) loadData() []tea.Cmd {
	m.newLoadContext()

	return []tea.Cmd{
		m.loadingTimer.Init(),
		m.beginTask(),
		func() tea.Msg {
			dataset, err := m.loader.LoadDataset(m.loadCtx, m.repoPath, m.environment)
			if err != nil {
				// Gen 0: the foundational Init load; never stale-dropped.
				return errMsg{err: err}
			}
			// Gen 0 is the always-apply sentinel (see handleDataMsg). This
			// foundational load is issued exactly once from Init and must
			// never be dropped as stale: when Init starts on a lazy-loaded
			// category it ALSO issues updateCategory, which bumps the
			// generation. A generationed dataset here would then look stale
			// (gen < current) and be dropped, blanking every non-lazy
			// category (e.g. definitions) on `toolkit -c <lazy-cat>`.
			return datasetLoadedMsg{Dataset: dataset, Gen: 0}
		},
	}
}

func setFilter(filter string) tea.Cmd {
	if filter == "" {
		return nil
	}

	return func() tea.Msg {
		return setFilterMsg(filter)
	}
}

// lazyLoadedCategories is a set of categories that are loaded on demand and never mutated.
var lazyLoadedCategories = map[domain.Category]struct{}{
	domain.BaseModel:          {},
	domain.ImportedModel:      {},
	domain.GPUPool:            {},
	domain.GPUNode:            {},
	domain.GPUWorkload:        {},
	domain.DedicatedAICluster: {},
}

// Init implements the tea.Model interface and initializes the model.
func (m *Model) Init() tea.Cmd {
	cmds := m.loadData()

	if _, ok := lazyLoadedCategories[m.category]; ok {
		cmds = append(cmds, m.updateCategory(m.category)...)
	}

	cmds = append(cmds, setFilter(m.initialFilter))
	// Establish the always-on working-tree watch in parallel with the initial
	// load, on the session context so navigation never cancels it.
	return tea.Batch(
		tea.Sequence(cmds...),
		startRepoWatchCmd(m.sessionCtx(), m.loader, m.repoPath),
	)
}
