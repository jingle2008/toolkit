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

func refreshDataCmd() tea.Cmd { return func() tea.Msg { return DataMsg{} } }

func (m *Model) getCompartmentID() (string, error) {
	if m.dataset != nil && m.dataset.GpuNodeMap != nil {
		for _, v := range m.dataset.GpuNodeMap {
			for _, n := range v {
				return n.CompartmentID, nil
			}
		}
	}

	clientset, err := k8s.NewClientsetFromKubeConfig(m.kubeConfig, m.environment.GetKubeContext())
	if err != nil {
		return "", err
	}
	nodes, err := k8s.ListGpuNodes(m.ctx, clientset, 1)
	if err != nil || len(nodes) == 0 {
		return "", err
	}

	return nodes[0].CompartmentID, nil
}

func (m *Model) updateGpuPoolState() tea.Cmd {
	return func() tea.Msg {
		var err error
		var compartmentID string
		if compartmentID, err = m.getCompartmentID(); err == nil {
			err = actions.PopulateGpuPools(m.ctx, m.dataset.GpuPools, m.environment, compartmentID)
		}
		return updateDoneMsg{err: err, category: domain.GpuPool}
	}
}

/*
updateRows updates the table rows based on the current model state.
Now also sets m.stats from getTableRows.
*/
func (m *Model) updateRows(autoSelect bool) {
	rows, stats := getTableRows(m.dataset, m.category, m.context, m.curFilter, m.sortColumn, m.sortAsc, m.showFaulty)
	m.stats = stats
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
		table.WithWidth(w - borderWidth)(m.table)
		table.WithHeight(h - borderHeight - top - 1)(m.table)
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
func (m *Model) processData(msg DataMsg) tea.Cmd {
	var cmd tea.Cmd
	// Drop stale dataset responses based on generation token
	if _, isDataset := msg.Data.(*models.Dataset); isDataset && msg.Gen != m.gen {
		return nil
	}
	switch data := msg.Data.(type) {
	case *models.Dataset:
		m.dataset = data
	case []models.BaseModel:
		m.dataset.BaseModels = data
	case []models.GpuPool:
		m.dataset.GpuPools = data
		cmd = m.updateGpuPoolState()
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
	return cmd
}

// Typed lazy-load handlers (replace DataMsg type-switch path)
// Each handler updates the dataset, ends the task, logs, refreshes display,
// and returns any follow-up command (e.g., GPU pool state enrichment).
func (m *Model) handleBaseModelsLoaded(items []models.BaseModel) tea.Cmd {
	m.dataset.BaseModels = items
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.BaseModel, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handleGpuPoolsLoaded(items []models.GpuPool) tea.Cmd {
	m.dataset.GpuPools = items
	cmd := m.updateGpuPoolState()
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.GpuPool, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return cmd
}

func (m *Model) handleGpuNodesLoaded(items map[string][]models.GpuNode) tea.Cmd {
	m.dataset.GpuNodeMap = items
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.GpuNode, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handleDedicatedAIClustersLoaded(items map[string][]models.DedicatedAICluster) tea.Cmd {
	m.dataset.SetDedicatedAIClusterMap(items)
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.DedicatedAICluster, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handleTenancyOverridesLoaded(group models.TenancyOverrideGroup) tea.Cmd {
	m.dataset.Tenants = group.Tenants
	m.dataset.LimitTenancyOverrideMap = group.LimitTenancyOverrideMap
	m.dataset.ConsolePropertyTenancyOverrideMap = group.ConsolePropertyTenancyOverrideMap
	m.dataset.PropertyTenancyOverrideMap = group.PropertyTenancyOverrideMap
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.Tenant, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handleLimitRegionalOverridesLoaded(items []models.LimitRegionalOverride) tea.Cmd {
	m.dataset.LimitRegionalOverrides = items
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.LimitRegionalOverride, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handleConsolePropertyRegionalOverridesLoaded(items []models.ConsolePropertyRegionalOverride) tea.Cmd {
	m.dataset.ConsolePropertyRegionalOverrides = items
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.ConsolePropertyRegionalOverride, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
}

func (m *Model) handlePropertyRegionalOverridesLoaded(items []models.PropertyRegionalOverride) tea.Cmd {
	m.dataset.PropertyRegionalOverrides = items
	m.endTask(true)
	m.logger.Infow("data loaded", "category", domain.PropertyRegionalOverride, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
	return nil
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

/*
handleAdditionalKeys processes extra key events for the current category.
Refactored to reduce cyclomatic complexity by extracting item actions.
*/
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

	return m.handleItemActions(msg)
}

// handleItemActions processes per-row actions for the current category.
func (m *Model) handleItemActions(msg tea.KeyMsg) tea.Cmd {
	itemKey := getItemKey(m.category, m.table.SelectedRow())
	item := findItem(m.dataset, m.category, itemKey)
	switch {
	case key.Matches(msg, keys.CopyTenant):
		actions.CopyTenantID(item, m.environment, m.logger)
	case key.Matches(msg, keys.Refresh):
		return tea.Sequence(m.updateCategoryNoHist(m.category)...)
	case key.Matches(msg, keys.ToggleCordon):
		return m.cordonNode(item)
	case key.Matches(msg, keys.DrainNode):
		return m.drainNode(item)
	case key.Matches(msg, keys.Delete):
		return m.deleteItem(itemKey)
	case key.Matches(msg, keys.RebootNode):
		return m.rebootNode(item)
	case key.Matches(msg, keys.ScaleUp):
		return m.scaleUpGpuPool(item)
	}
	return nil
}

func (m *Model) scaleUpGpuPool(item any) tea.Cmd {
	pool, ok := item.(*models.GpuPool)
	if !ok || pool == nil {
		m.logger.Errorw("no GPU pool selected for scale up")
		return nil
	}

	key := getItemKey(m.category, m.table.SelectedRow())
	return tea.Batch(
		func() tea.Msg { return gpuPoolScaleStartedMsg{key: key} },
		func() tea.Msg {
			err := actions.IncreasePoolSize(m.ctx, pool, m.environment, m.logger)
			return gpuPoolScaleResultMsg{key: key, err: err}
		},
	)
}

func (m *Model) toggleFaultyList() tea.Cmd {
	m.showFaulty = !m.showFaulty
	m.updateRows(true)
	return nil
}

func (m *Model) cordonNode(item any) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for cordon operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GpuNode)
	if !ok {
		m.logger.Errorw("unsupported item type for cordon operation", "item", item)
		return nil
	}
	key := getItemKey(m.category, m.table.SelectedRow())
	return func() tea.Msg {
		state, err := k8s.ToggleCordon(m.ctx, m.kubeConfig, m.environment.GetKubeContext(), node.Name)
		return cordonNodeResultMsg{key: key, state: state, err: err}
	}
}

func (m *Model) drainNode(item any) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for draining", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GpuNode)
	if !ok {
		m.logger.Errorw("unsupported item type for draining", "item", item)
		return nil
	}
	key := getItemKey(m.category, m.table.SelectedRow())
	return func() tea.Msg {
		err := k8s.DrainNode(m.ctx, m.kubeConfig, m.environment.GetKubeContext(), node.Name)
		return drainNodeResultMsg{key: key, err: err}
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
		domain.GpuPool:                         func(m *Model, refresh bool) tea.Cmd { return m.handleGpuPoolCategory(refresh) },
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
		m.newLoadContext()
		cmds = append(cmds, m.beginTask(), cmd)
	} else {
		cmds = append(cmds, refreshDataCmd())
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
		return loadRequest{
			category:    domain.Tenant,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleLimitRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.LimitRegionalOverrides == nil {
		return loadRequest{
			category:    domain.LimitRegionalOverride,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleConsolePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.ConsolePropertyRegionalOverrides == nil {
		return loadRequest{
			category:    domain.ConsolePropertyRegionalOverride,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handlePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.PropertyRegionalOverrides == nil {
		return loadRequest{
			category:    domain.PropertyRegionalOverride,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleBaseModelCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModels == nil {
		return loadRequest{
			category:    domain.BaseModel,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleGpuPoolCategory(refresh bool) tea.Cmd {
	if m.dataset == nil || m.dataset.GpuPools == nil || refresh {
		return loadRequest{
			category:    domain.GpuPool,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleGpuNodeCategory(refresh bool) tea.Cmd {
	if m.dataset == nil || m.dataset.GpuNodeMap == nil || refresh {
		return loadRequest{
			category:    domain.GpuNode,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
	}
	return nil
}

func (m *Model) handleDedicatedAIClusterCategory(refresh bool) tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil || refresh {
		return loadRequest{
			category:    domain.DedicatedAICluster,
			loader:      m.loader,
			ctx:         m.loadCtx,
			repoPath:    m.repoPath,
			kubeConfig:  m.kubeConfig,
			environment: m.environment,
		}.Run
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
	m.updateLayout(m.viewWidth, m.viewHeight)
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
		cmd = tea.Sequence(
			m.loadingSpinner.Tick, // start the spinner
			m.loadingTimer.Reset(),
			m.loadingTimer.Start(), // start the stopwatch
		)
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
		elapsed := m.loadingTimer.Elapsed().String()
		m.logger.Infow("data load completed",
			"category", m.category,
			"success", success,
			"elapsed", elapsed,
		)
		if success {
			m.viewMode = m.lastViewMode
		} else {
			m.viewMode = common.ErrorView
		}
	}
}
