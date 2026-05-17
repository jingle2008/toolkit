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
	}

	// Dispatch table for category handlers
	type handlerFn func(*Model, bool, int) tea.Cmd
	handlers := map[domain.Category]handlerFn{
		domain.BaseModel:                       func(m *Model, _ bool, gen int) tea.Cmd { return m.handleBaseModelCategory(gen) },
		domain.GpuPool:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGpuPoolCategory(refresh, gen) },
		domain.GpuNode:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGpuNodeCategory(refresh, gen) },
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
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.newLoadContext()
	if fn, ok := handlers[m.category]; ok {
		gen := m.bumpGen()
		cmd = fn(m, refresh, gen)
	} else if _, ok := tenancyOverrides[m.category]; ok {
		gen := m.bumpGen()
		cmd = m.handleTenancyOverridesGroup(gen)
	}
	if cmd != nil {
		cmds = append(cmds, m.beginTask(), cmd)
	} else {
		cmds = append(cmds, refreshDataCmd())
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

func (m *Model) handleBaseModelCategory(gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModels == nil {
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleGpuPoolCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GpuPools == nil || refresh {
		return loadGpuPoolsCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	}
	return nil
}

func (m *Model) handleGpuNodeCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GpuNodeMap == nil || refresh {
		return loadGpuNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
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
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}

	m.viewMode = common.DetailsView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.choice = getItemKey(m.category, row)
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

// enterContext moves the model into a new context based on the selected row.
func (m *Model) enterContext() tea.Cmd {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}

	target := row[0]
	switch {
	case m.category.IsScope():
		m.context = &domain.ToolkitContext{Category: m.category, Name: target}
		return tea.Sequence(m.updateCategory(m.category.ScopedCategories()[0])...)
	case m.category == domain.Environment:
		env := *collections.FindByName(m.dataset.Environments, target)
		if !m.environment.Equals(env) {
			m.environment = env
			m.dataset.ResetScopedData()
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
