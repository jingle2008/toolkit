package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

/*
updateCategory changes the current category and loads data if needed.
This version records the navigation in history.
*/
func (m *Model) updateCategory(category domain.Category) []tea.Cmd {
	cmds := m.updateCategoryCore(category)
	m.pushHistory(category)
	return cmds
}

/*
updateCategoryNoHist changes the current category and loads data if needed,
but does NOT record the navigation in history.
*/
func (m *Model) updateCategoryNoHist(category domain.Category) []tea.Cmd {
	return m.updateCategoryCore(category)
}

/*
updateCategoryCore contains the shared logic for changing category.
*/
func (m *Model) updateCategoryCore(category domain.Category) []tea.Cmd {
	refresh := false
	if m.category == category {
		refresh = true
	} else {
		m.category = category
		m.keys = keys.ResolveKeys(m.category, m.viewMode)
		m.sortColumn = common.NameCol
		m.sortAsc = true
		m.showFaulty = false
		m.watching = false
		m.watchTrigger = nil
		// Filtering and cursor position are view state tied to the category
		// being browsed. Clear the filter here, on navigation, so an in-place
		// data refresh (refreshDisplay) can preserve it for the same category.
		m.filter = ""
		m.textInput.Reset()
		// Switch the visible chrome to the destination immediately so
		// the user sees what they navigated to (new headers, empty
		// rows) instead of stale data under a mismatched label.
		// refreshDisplay will repopulate rows once the load lands.
		m.updateColumns()
		m.applyRows(nil, nil, false)
	}

	// Dispatch table for category handlers
	type handlerFn func(*Model, bool, int) tea.Cmd
	handlers := map[domain.Category]handlerFn{
		domain.BaseModel:                       func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleBaseModelCategory(refresh, gen) },
		domain.ImportedModel:                   func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleImportedModelCategory(refresh, gen) },
		domain.GPUPool:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUPoolCategory(refresh, gen) },
		domain.GPUNode:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUNodeCategory(refresh, gen) },
		domain.GPUWorkload:                     func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUWorkloadCategory(refresh, gen) },
		domain.DedicatedAICluster:              func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleDedicatedAIClusterCategory(refresh, gen) },
		domain.LimitRegionalOverride:           func(m *Model, _ bool, gen int) tea.Cmd { return m.handleLimitRegionalOverrideCategory(gen) },
		domain.ConsolePropertyRegionalOverride: func(m *Model, _ bool, gen int) tea.Cmd { return m.handleConsolePropertyRegionalOverrideCategory(gen) },
		domain.PropertyRegionalOverride:        func(m *Model, _ bool, gen int) tea.Cmd { return m.handlePropertyRegionalOverrideCategory(gen) },
	}

	// Grouped handler for tenancy overrides
	tenancyOverrides := map[domain.Category]struct{}{
		domain.Tenant:                         {},
		domain.LimitTenancyOverride:           {},
		domain.ConsolePropertyTenancyOverride: {},
		domain.PropertyTenancyOverride:        {},
	}

	var (
		cmd      tea.Cmd
		watchCmd tea.Cmd
		cmds     []tea.Cmd
	)

	m.newLoadContext()
	if fn, ok := handlers[m.category]; ok {
		gen := m.bumpGen()
		cmd = fn(m, refresh, gen)
		if m.category.NeedsKubeConfig() {
			watchCmd = startWatchCmd(m.loadCtx, m.loader, m.category, m.kubeConfig, m.environment, gen)
		}
	} else if _, ok := tenancyOverrides[m.category]; ok {
		gen := m.bumpGen()
		cmd = m.handleTenancyOverridesGroup(gen)
	}
	if cmd != nil {
		cmds = append(cmds, m.beginTask(), cmd)
	} else {
		cmds = append(cmds, refreshDataCmd())
	}
	if watchCmd != nil {
		cmds = append(cmds, watchCmd)
	}
	return cmds
}

// Lazy loaders for realm-specific categories
func (m *Model) handleTenancyOverridesGroup(gen int) tea.Cmd {
	if m.dataset == nil ||
		m.dataset.Tenants == nil ||
		m.dataset.LimitTenancyOverrideMap == nil ||
		m.dataset.ConsolePropertyTenancyOverrideMap == nil ||
		m.dataset.PropertyTenancyOverrideMap == nil {
		return loadTenancyOverrideGroupCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handleLimitRegionalOverrideCategory(gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.LimitRegionalOverrides == nil {
		return loadLimitRegionalOverridesCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handleConsolePropertyRegionalOverrideCategory(gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.ConsolePropertyRegionalOverrides == nil {
		return loadConsolePropertyRegionalOverridesCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handlePropertyRegionalOverrideCategory(gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.PropertyRegionalOverrides == nil {
		return loadPropertyRegionalOverridesCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handleBaseModelCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModels == nil || refresh {
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleImportedModelCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.ImportedModelMap == nil || refresh {
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleGPUPoolCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GPUPools == nil || refresh {
		return loadGPUPoolsCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handleGPUNodeCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GPUNodeMap == nil || refresh {
		return loadGPUNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleGPUWorkloadCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GPUWorkloadMap == nil || refresh {
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleDedicatedAIClusterCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil || refresh {
		return loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

// enterDetailView switches the model into detail view mode.
func (m *Model) enterDetailView() tea.Cmd {
	row := m.selectedRawRow()
	if len(row) == 0 {
		return nil
	}

	m.viewMode = common.DetailsView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.selectedKey = itemKeyFrom(m.category, row)
	m.updateLayout(m.viewWidth, m.viewHeight)
	return m.updateContentAsync()
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.updateLayout(m.viewWidth, m.viewHeight)
}

// changeCategory parses the text input and updates the category.
func (m *Model) changeCategory() tea.Cmd {
	text := m.textInput.Value()
	category, err := domain.ParseCategory(text)
	if err != nil {
		return nil
	}

	if m.category == category {
		return nil
	}
	return tea.Sequence(m.updateCategory(category)...)
}

// jumpToParent navigates from a sub-category back to its parent category
// and re-selects the parent row. The scope is kept/set so applyRows'
// auto-select highlights the parent (computeTableRows ignores a scope
// that does not scope its own category, so the full parent list shows).
//
// When we drilled in from a parent, the existing scope identifies the
// parent unambiguously (it even disambiguates the tenancy overrides, which
// have both a Tenant and a Definition parent). Otherwise — a sub-category
// reached directly via tab/command with no context selected — the parent is
// derived from the selected row, but only when it is unambiguous. It is a
// no-op for categories that have no single parent.
func (m *Model) jumpToParent() tea.Cmd {
	if m.scope != nil && m.scope.Category.IsScopeOf(m.category) {
		return tea.Sequence(m.updateCategory(m.scope.Category)...)
	}
	parent, ok := parentScope(m.category, m.selectedRawRow())
	if !ok {
		return nil
	}
	m.scope = &parent
	return tea.Sequence(m.updateCategory(parent.Category)...)
}

// enterContext moves the model into a new context based on the selected row.
func (m *Model) enterContext() tea.Cmd {
	row := m.selectedRawRow()
	if len(row) == 0 {
		return nil
	}

	target := row[0]
	switch {
	case m.category.IsScope():
		m.scope = &domain.Scope{Category: m.category, Name: target}
		return tea.Sequence(m.updateCategory(m.category.ScopedCategories()[0])...)
	case m.category == domain.Environment:
		envPtr := collections.FindByName(m.dataset.Environments, target)
		if envPtr == nil {
			// Selected row doesn't match any known environment (e.g.
			// stale table state). Nothing to do.
			return nil
		}
		if !m.environment.Equals(*envPtr) {
			m.environment = *envPtr
			m.dataset.ResetRealmScopedFields()
			return tea.Sequence(m.updateCategory(domain.Tenant)...)
		}
	case m.category == domain.Alias:
		if cat, _ := domain.ParseCategory(target); cat != m.category {
			return tea.Sequence(m.updateCategory(cat)...)
		}
	default:
		return m.enterDetailView()
	}
	return nil
}
