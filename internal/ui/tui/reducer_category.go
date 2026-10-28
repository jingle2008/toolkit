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
	if src, ok := lazySources[m.category]; ok {
		gen := m.bumpGen()
		if m.category.NeedsKubeConfig() {
			cached = !refresh
			refresh = true
			watchCmd = startK8sWatchCmd(m.loadCtx, m.loader, m.category, m.kubeConfig, m.environment, gen)
		}
		cmd = m.categoryLoad(m.category, src, refresh, gen)
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
	// One batch, so a caller's tea.Sequence runs these together: in order,
	// the cached rows would wait for the reload they're meant to cover, and
	// the watch would start only after it. The order they land in doesn't
	// matter, as the redraw reads whatever the dataset holds by then.
	return []tea.Cmd{tea.Batch(cmds...)}
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

// enterDetailView switches the model into detail view mode.
func (m *Model) enterDetailView() tea.Cmd {
	row := m.selectedRawRow()
	if len(row) == 0 {
		return nil
	}

	m.setBaseView(common.DetailsView)
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.selectedKey = itemKeyFrom(m.category, row)
	m.updateLayout(m.viewWidth, m.viewHeight)
	m.detailStale, m.detailItem, m.detailGone = false, nil, false
	return m.updateContentAsync()
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.setBaseView(common.ListView)
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
