package tui

import (
	tea "charm.land/bubbletea/v2"

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
	m.saveView()
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

// handlerFn loads/refreshes one category. Package-scoped so drift guards can
// assert coverage and so the map isn't rebuilt on every navigation.
type handlerFn func(*Model, bool, int) tea.Cmd

var categoryHandlers = map[domain.Category]handlerFn{
	domain.BaseModel:                       func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleBaseModelCategory(refresh, gen) },
	domain.ServingRuntime:                  func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleServingRuntimeCategory(refresh, gen) },
	domain.ImportedModel:                   func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleImportedModelCategory(refresh, gen) },
	domain.GPUPool:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUPoolCategory(refresh, gen) },
	domain.GPUNode:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUNodeCategory(refresh, gen) },
	domain.GPUWorkload:                     func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUWorkloadCategory(refresh, gen) },
	domain.DedicatedAICluster:              func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleDedicatedAIClusterCategory(refresh, gen) },
	domain.InferenceService:                func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleInferenceServiceCategory(refresh, gen) },
	domain.LimitRegionalOverride:           func(m *Model, _ bool, gen int) tea.Cmd { return m.handleLimitRegionalOverrideCategory(gen) },
	domain.ConsolePropertyRegionalOverride: func(m *Model, _ bool, gen int) tea.Cmd { return m.handleConsolePropertyRegionalOverrideCategory(gen) },
	domain.PropertyRegionalOverride:        func(m *Model, _ bool, gen int) tea.Cmd { return m.handlePropertyRegionalOverrideCategory(gen) },
}

var tenancyOverrideCategories = map[domain.Category]struct{}{
	domain.Tenant:                         {},
	domain.LimitTenancyOverride:           {},
	domain.ConsolePropertyTenancyOverride: {},
	domain.PropertyTenancyOverride:        {},
}

// tenantOwnedCategories are the cluster categories whose rows name their
// owning tenant, resolved from Dataset.Tenants: without the tenants they
// show raw tenancy IDs.
var tenantOwnedCategories = map[domain.Category]struct{}{
	domain.DedicatedAICluster: {},
	domain.ImportedModel:      {},
	domain.GPUWorkload:        {},
	domain.InferenceService:   {},
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
		m.landOn, m.landOnKey = nil, nil // cursor targets for an earlier navigation
		// A scope carries over to its other children (:gw after drilling
		// into a DAC) and to its own category (the row to land on), but
		// not past a category it doesn't apply to: there it would be
		// hidden, esc couldn't clear it, and the next child would come
		// back scoped to it.
		if m.scope != nil && m.scope.Category != category && !m.scope.Category.IsScopeOf(category) {
			m.scope = nil
		}
		m.keys = keys.ResolveKeys(m.category, m.viewMode)
		m.sortColumn = common.NameCol
		m.sortAsc = true
		m.showFaulty = false
		m.watch.k8sActive = false
		m.watch.k8sTrigger = nil
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

	var (
		cmd      tea.Cmd
		watchCmd tea.Cmd
		cmds     []tea.Cmd
	)

	m.newLoadContext()
	m.loadFailed = false
	// A cluster category's cache went stale when its watch stopped, as it
	// does on leaving the category, and the new watch only reports changes
	// from now on. So returning to one reloads it, showing the cached rows
	// (cached) until the fresh ones land.
	var cached bool
	if fn, ok := categoryHandlers[m.category]; ok {
		gen := m.bumpGen()
		if m.category.NeedsKubeConfig() {
			cached = !refresh
			refresh = true
			watchCmd = startK8sWatchCmd(m.loadCtx, m.loader, m.category, m.kubeConfig, m.environment, gen)
		}
		cmd = fn(m, refresh, gen)
	} else if _, ok := tenancyOverrideCategories[m.category]; ok {
		gen := m.bumpGen()
		cmd = m.handleTenancyOverridesGroup(gen)
	}
	if cmd != nil {
		cmds = append(cmds, m.beginTask(), cmd)
	}
	if cmd == nil || cached {
		cmds = append(cmds, refreshDataCmd())
	}
	if tenants := m.tenantsForOwnersCmd(); tenants != nil {
		cmds = append(cmds, m.beginTask(), tenants)
	}
	if watchCmd != nil {
		cmds = append(cmds, watchCmd)
	}
	return cmds
}

// tenantsForOwnersCmd loads the tenancy group when a tenant-owned category
// is showing without tenants, as after a switch to another realm, which
// drops them; handleTenancyOverridesLoaded resolves the owners and redraws.
// It shares the gen of the category's own load.
func (m *Model) tenantsForOwnersCmd() tea.Cmd {
	if _, ok := tenantOwnedCategories[m.category]; !ok || m.dataset == nil || m.dataset.Tenants != nil {
		return nil
	}
	return loadTenancyOverrideGroupCmd(m.loadCtx, m.loader, m.repoPath, m.environment, m.gens.msg)
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

func (m *Model) handleServingRuntimeCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.ServingRuntimes == nil || refresh {
		return loadServingRuntimesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
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
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen, m.workloadsNeedDACs())
	}
	return nil
}

func (m *Model) handleDedicatedAIClusterCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil || refresh {
		return loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}

func (m *Model) handleInferenceServiceCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.InferenceServiceMap == nil || refresh {
		return loadInferenceServicesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
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
	m.detailStale, m.detailItem, m.detailGone = false, nil, false
	return m.updateContentAsync()
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.updateLayout(m.viewWidth, m.viewHeight)
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
		return tea.Sequence(m.updateCategory(m.drillTarget())...)
	case m.category == domain.Environment:
		envPtr := collections.FindByName(m.dataset.Environments, target)
		if envPtr == nil {
			// Selected row doesn't match any known environment (e.g.
			// stale table state). Nothing to do.
			return nil
		}
		if !m.environment.Equals(*envPtr) {
			return m.switchEnvironment(*envPtr, domain.Tenant)
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

// drillTarget is the child category Enter drills into under m.scope: the
// first, in registry order, with rows for the scope in what's loaded, so
// Enter on a tenant without limit overrides lands on its console
// properties rather than an empty list. With none, the first child: an
// unloaded category may still have rows, and loading it finds out.
func (m *Model) drillTarget() domain.Category {
	children := m.category.ScopedCategories()
	for _, c := range children {
		if rows, _ := computeTableRows(m.dataset, c, m.scope, "", "", true, false); len(rows) > 0 {
			return c
		}
	}
	return children[0]
}
