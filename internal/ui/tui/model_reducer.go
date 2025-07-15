// Package tui contains reducer and event logic for the Model.
// This file contains methods for state transitions, event handling, and UI updates.
package tui

import (
	"math"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

var refreshDataCmd tea.Cmd = func() tea.Msg { return DataMsg{} }

// updateRows updates the table rows based on the current model state.
func (m *Model) updateRows(autoSelect bool) {
	rows := getTableRows(m.logger, m.dataset, m.category, m.context, m.curFilter, m.sortColumn, m.sortAsc, m.showFaulty)
	table.WithRows(rows)(m.table)

	if autoSelect {
		idx := m.findContextIndex(rows)
		if idx >= 0 {
			m.table.SetCursor(idx)
		} else {
			m.table.GotoTop()
		}
	} else {
		m.table.UpdateViewport()
	}
}

// updateColumns updates the table columns based on the current category.
func (m *Model) updateColumns() {
	m.headers = getHeaders(m.category)
	columns := make([]table.Column, len(m.headers))
	remaining := m.table.Width()
	for i, header := range m.headers {
		width := remaining
		if i+1 < len(m.headers) {
			width = int(math.Floor(float64(m.table.Width()) * float64(header.ratio)))
			remaining -= width
		}
		width -= m.styles.Header.GetHorizontalFrameSize()
		title := header.text
		if m.sortColumn != "" && strings.EqualFold(header.text, m.sortColumn) {
			if m.sortAsc {
				title += " ↑"
			} else {
				title += " ↓"
			}
		}
		columns[i] = table.Column{Title: title, Width: width}
	}
	table.WithColumns(columns)(m.table)
}

// findContextIndex returns the index of the row to move the cursor to, based on environment or context.
func (m *Model) findContextIndex(rows []table.Row) int {
	if len(rows) == 0 {
		return -1
	}

	var name string
	switch {
	case m.category == domain.Environment:
		name = m.environment.GetName()
	case m.context != nil && m.category == m.context.Category:
		name = m.context.Name
	default:
		return -1
	}

	for i, r := range rows {
		if r[0] == name {
			return i
		}
	}

	return -1
}

// updateLayout recalculates the layout for the view and table.
func (m *Model) updateLayout(w, h int) {
	m.viewWidth, m.viewHeight = w, h
	m.help.Width = w
	var borderStyle lipgloss.Border
	if m.viewMode == common.DetailsView {
		borderStyle = m.viewport.Style.GetBorderStyle()
	} else {
		borderStyle = m.baseStyle.GetBorderStyle()
	}
	borderWidth := borderStyle.GetLeftSize() + borderStyle.GetRightSize()
	borderHeight := borderStyle.GetTopSize() + borderStyle.GetBottomSize()
	statusHeight := lipgloss.Height(m.statusView())
	infoHeight := lipgloss.Height(m.infoView())
	top := statusHeight + infoHeight
	if m.viewMode == common.DetailsView {
		m.viewport.Width = w
		m.viewport.Height = h - top
		m.updateContent(w - borderWidth)
	} else {
		headerHeight := lipgloss.Height(m.styles.Header.Render("test"))
		table.WithWidth(w - borderWidth)(m.table)
		table.WithHeight(h - borderHeight - headerHeight - top)(m.table)
		m.updateColumns()
		m.table.UpdateViewport()
	}
}

// refreshDisplay resets filters and updates columns and rows.
func (m *Model) refreshDisplay() {
	m.curFilter = ""
	m.newFilter = ""
	m.textInput.Reset()
	m.updateColumns()
	m.updateRows(true)
}

// processData updates the model's dataset based on the incoming DataMsg.
func (m *Model) processData(msg DataMsg) {
	switch data := msg.Data.(type) {
	case *models.Dataset:
		m.dataset = data
	case map[string]*models.BaseModel:
		m.dataset.BaseModelMap = data
	case []models.GpuPool:
		m.dataset.GpuPools = data
	case map[string][]models.GpuNode:
		m.dataset.GpuNodeMap = data
	case map[string][]models.DedicatedAICluster:
		m.dataset.SetDedicatedAIClusterMap(data)
	case models.TenancyOverrideGroup:
		m.dataset.Tenants = data.Tenants
		m.dataset.LimitTenancyOverrideMap = data.LimitTenancyOverrideMap
		m.dataset.ConsolePropertyTenancyOverrideMap = data.ConsolePropertyTenancyOverrideMap
		m.dataset.PropertyTenancyOverrideMap = data.PropertyTenancyOverrideMap
	}

	if msg.Data != nil {
		m.endTask(true)
		m.logger.Infow("data loaded", "category", m.category, "pendingTasks", m.pendingTasks)
	}
	m.refreshDisplay()
}

func (m *Model) sortTableByColumn(column string) tea.Cmd {
	if m.sortColumn == column {
		m.sortAsc = !m.sortAsc
	} else {
		m.sortColumn = column
		m.sortAsc = true
	}

	m.updateColumns()
	m.updateRows(true)
	return nil
}

// handleAdditionalKeys processes extra key events for the current category.
func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) tea.Cmd {
	idx := slices.IndexFunc(m.keys.Context, func(b key.Binding) bool {
		return key.Matches(msg, b)
	})

	if idx < 0 {
		return nil
	}

	binding := m.keys.Context[idx]
	if column, ok := strings.CutPrefix(binding.Help().Desc, keys.SortPrefix); ok {
		return m.sortTableByColumn(column)
	}

	if key.Matches(msg, keys.ToggleFaulty) {
		return m.toggleFaultyList()
	}

	item := m.getSelectedItem()
	switch {
	case key.Matches(msg, keys.CopyTenant):
		actions.CopyTenantID(item, &m.environment, m.logger)
	case key.Matches(msg, keys.Refresh):
		return tea.Sequence(m.updateCategory(m.category)...)
	case key.Matches(msg, keys.ToggleCordon):
		m.cordonNode(item)
	case key.Matches(msg, keys.DrainNode):
		m.drainNode(item)
	}

	return nil
}

func (m *Model) toggleFaultyList() tea.Cmd {
	// Only allow toggle if no context and no filter
	if m.context != nil || m.curFilter != "" {
		return nil
	}
	m.showFaulty = !m.showFaulty
	return tea.Sequence(m.updateCategoryNoHist(m.category)...)
}

func (m *Model) cordonNode(item any) {
	if item == nil {
		m.logger.Errorw("no item selected for cordon operation", "category", m.category)
		return
	}

	if node, ok := item.(*models.GpuNode); ok {
		if state, err := k8s.ToggleCordon(m.ctx, m.kubeConfig, m.environment.GetKubeContext(), node.Name); err != nil {
			m.logger.Errorw("failed to toggle cordon state", "error", err)
		} else {
			node.IsSchedulingDisabled = state
			m.updateRows(false)
		}
	} else {
		m.logger.Errorw("unsupported item type for cordon operation", "item", item)
	}
}

func (m *Model) drainNode(item any) {
	if item == nil {
		m.logger.Errorw("no item selected for draining", "category", m.category)
		return
	}

	if node, ok := item.(*models.GpuNode); ok {
		if err := k8s.DrainNode(m.ctx, m.kubeConfig, m.environment.GetKubeContext(), node.Name); err != nil {
			m.logger.Errorw("failed to drain node", "error", err)
		}
	} else {
		m.logger.Errorw("unsupported item type for draining", "item", item)
	}
}

// getSelectedItem returns the currently selected item in the table.
func (m *Model) getSelectedItem() any {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}

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
	type handlerFn func(*Model, bool) tea.Cmd
	handlers := map[domain.Category]handlerFn{
		domain.BaseModel:                       func(m *Model, _ bool) tea.Cmd { return m.handleBaseModelCategory() },
		domain.GpuPool:                         func(m *Model, _ bool) tea.Cmd { return m.handleGpuPoolCategory() },
		domain.GpuNode:                         func(m *Model, refresh bool) tea.Cmd { return m.handleGpuNodeCategory(refresh) },
		domain.DedicatedAICluster:              func(m *Model, refresh bool) tea.Cmd { return m.handleDedicatedAIClusterCategory(refresh) },
		domain.LimitRegionalOverride:           func(m *Model, _ bool) tea.Cmd { return m.handleLimitRegionalOverrideCategory() },
		domain.ConsolePropertyRegionalOverride: func(m *Model, _ bool) tea.Cmd { return m.handleConsolePropertyRegionalOverrideCategory() },
		domain.PropertyRegionalOverride:        func(m *Model, _ bool) tea.Cmd { return m.handlePropertyRegionalOverrideCategory() },
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
	if fn, ok := handlers[m.category]; ok {
		cmd = fn(m, refresh)
	} else if _, ok := tenancyOverrides[m.category]; ok {
		cmd = m.handleTenancyOverridesGroup()
	}
	if cmd != nil {
		cmds = append(cmds, m.beginTask(), cmd)
	} else {
		cmds = append(cmds, refreshDataCmd)
	}
	return cmds
}

// Lazy loaders for realm-specific categories
func (m *Model) handleTenancyOverridesGroup() tea.Cmd {
	if m.dataset == nil ||
		m.dataset.Tenants == nil ||
		m.dataset.LimitTenancyOverrideMap == nil ||
		m.dataset.ConsolePropertyTenancyOverrideMap == nil ||
		m.dataset.PropertyTenancyOverrideMap == nil {
		return loadRequest{category: domain.Tenant, model: m}.Run
	}
	return nil
}

func (m *Model) handleLimitRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.LimitRegionalOverrides == nil {
		return loadRequest{category: domain.LimitRegionalOverride, model: m}.Run
	}
	return nil
}

func (m *Model) handleConsolePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.ConsolePropertyRegionalOverrides == nil {
		return loadRequest{category: domain.ConsolePropertyRegionalOverride, model: m}.Run
	}
	return nil
}

func (m *Model) handlePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.PropertyRegionalOverrides == nil {
		return loadRequest{category: domain.PropertyRegionalOverride, model: m}.Run
	}
	return nil
}

func (m *Model) handleBaseModelCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModelMap == nil {
		return loadRequest{category: domain.BaseModel, model: m}.Run
	}
	return nil
}

func (m *Model) handleGpuPoolCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.GpuPools == nil {
		return loadRequest{category: domain.GpuPool, model: m}.Run
	}
	return nil
}

func (m *Model) handleGpuNodeCategory(refresh bool) tea.Cmd {
	if m.dataset == nil || m.dataset.GpuNodeMap == nil || refresh {
		return loadRequest{category: domain.GpuNode, model: m}.Run
	}
	return nil
}

func (m *Model) handleDedicatedAIClusterCategory(refresh bool) tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil || refresh {
		return loadRequest{category: domain.DedicatedAICluster, model: m}.Run
	}
	return nil
}

// enterDetailView switches the model into detail view mode.
func (m *Model) enterDetailView() {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return
	}

	m.viewMode = common.DetailsView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.choice = getItemKey(m.category, row)
	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	} else {
		m.updateContent(0)
	}
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	}
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
		m.enterDetailView()
	}
	return nil
}

/*
beginTask increments the pendingTasks counter and switches to LoadingView if needed.
Call this before starting an async task.
*/
func (m *Model) beginTask() tea.Cmd {
	var cmd tea.Cmd
	if m.pendingTasks == 0 {
		m.lastViewMode = m.viewMode
		m.viewMode = common.LoadingView
		cmd = m.loadingSpinner.Tick // start the spinner
	}
	m.pendingTasks++
	return cmd
}

/*
endTask decrements the pendingTasks counter and restores the previous view mode if all tasks are done.
Call this after an async task completes.
*/
func (m *Model) endTask(success bool) {
	if m.pendingTasks > 0 {
		m.pendingTasks--
	}
	if m.pendingTasks == 0 {
		if success {
			m.viewMode = m.lastViewMode
		} else {
			m.viewMode = common.ErrorView
		}
	}
}
