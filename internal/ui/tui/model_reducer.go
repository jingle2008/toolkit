// Package tui contains reducer and event logic for the Model.
// This file contains methods for state transitions, event handling, and UI updates.
package tui

import (
	"context"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

func refreshDataCmd() tea.Cmd { return func() tea.Msg { return DataMsg{} } }

func (m *Model) getCompartmentID(ctx context.Context) (string, error) {
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
	nodes, err := k8s.ListGpuNodes(ctx, clientset, 1)
	if err != nil || len(nodes) == 0 {
		return "", err
	}

	return nodes[0].CompartmentID, nil
}

func (m *Model) updateGpuPoolState() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := m.opContext()
		defer cancel()
		var err error
		var compartmentID string
		if compartmentID, err = m.getCompartmentID(ctx); err == nil {
			err = actions.PopulateGpuPools(ctx, m.dataset.GpuPools, m.environment, compartmentID)
		}
		return updateDoneMsg{err: err, category: domain.GpuPool}
	}
}

/*
updateRows updates the table rows based on the current model state.
Now also sets m.stats from getTableRows.
*/
func (m *Model) updateRows(autoSelect bool) {
	m.rowsNonce++
	rows, stats := getTableRows(m.dataset, m.category, m.context, m.curFilter, m.sortColumn, m.sortAsc, m.showFaulty)
	m.applyRows(rows, stats, autoSelect)
}

func (m *Model) updateRowsAsync() tea.Cmd {
	m.rowsNonce++
	nonce := m.rowsNonce
	dataset := m.dataset
	category := m.category
	context := m.context
	filter := m.curFilter
	sortColumn := m.sortColumn
	sortAsc := m.sortAsc
	showFaulty := m.showFaulty
	return func() tea.Msg {
		rows, stats := getTableRows(dataset, category, context, filter, sortColumn, sortAsc, showFaulty)
		return tableRowsComputedMsg{
			Rows:  rows,
			Stats: stats,
			Nonce: nonce,
		}
	}
}

func (m *Model) handleTableRowsComputedMsg(msg tableRowsComputedMsg) {
	if msg.Nonce != m.rowsNonce {
		return
	}
	m.applyRows(msg.Rows, msg.Stats, true)
}

func (m *Model) applyRows(rows []table.Row, stats tableStats, autoSelect bool) {
	m.stats = stats
	m.applyMiddleTruncation(rows)
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

// applyMiddleTruncation shortens cells in columns marked
// TruncateMiddle so values wider than their column's display width
// are elided in the MIDDLE (head + "…" + tail) instead of having
// their tail chopped by bubbles' default right-truncation. After
// this pass the cell's measured width is ≤ the column width, so
// runewidth.Truncate inside bubbles becomes a no-op for these
// cells. Rows are mutated in place.
//
// Columns marked TruncateMiddle today hold OCID-suffix values (DAC
// and ImportedModel Name; the Tenant key on those plus tenancy-
// override grouped sets); the head identifies the OCID shape while
// the tail is the distinguishing portion — both worth keeping.
func (m *Model) applyMiddleTruncation(rows []table.Row) {
	if len(rows) == 0 || len(m.headers) == 0 {
		return
	}
	cols := m.table.Columns()
	for c, h := range m.headers {
		if !h.truncateMiddle || c >= len(cols) {
			continue
		}
		w := cols[c].Width
		if w <= 0 {
			continue
		}
		for r := range rows {
			if c >= len(rows[r]) {
				continue
			}
			rows[r][c] = truncateMiddle(rows[r][c], w)
		}
	}
}

// truncateMiddle returns s shortened to fit width w by eliding the
// middle: head + "…" + tail, where head and tail are sized to use
// the available width minus the ellipsis. The split favors the tail
// by one cell when w-1 is odd, since the tail is typically the
// distinguishing portion for OCID-shaped values. If s already fits,
// it is returned unchanged.
func truncateMiddle(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	const ellipsis = "…"
	ew := runewidth.StringWidth(ellipsis)
	if w <= 0 {
		return ""
	}
	if w < ew {
		return ""
	}
	if w == ew {
		return ellipsis
	}
	keep := w - ew
	headW := keep / 2
	tailW := keep - headW

	runes := []rune(s)
	headEnd := 0
	var acc int
	for i := 0; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if acc+rw > headW {
			break
		}
		acc += rw
		headEnd = i + 1
	}
	tailStart := len(runes)
	acc = 0
	for i := len(runes) - 1; i >= 0; i-- {
		rw := runewidth.RuneWidth(runes[i])
		if acc+rw > tailW {
			break
		}
		acc += rw
		tailStart = i
	}
	if tailStart < headEnd {
		tailStart = headEnd
	}
	return string(runes[:headEnd]) + ellipsis + string(runes[tailStart:])
}

// updateColumns updates the table columns based on the current category.
func (m *Model) updateColumns() {
	m.headers = getHeaders(m.category)
	sortable := m.keys.SortableColumns()
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
		switch {
		case m.sortColumn != "" && strings.EqualFold(header.text, m.sortColumn):
			if m.sortAsc {
				title += " ↑"
			} else {
				title += " ↓"
			}
		case sortable[strings.ToLower(header.text)]:
			title += " ↕"
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
	// Budget the info column out of the help bubble's width so the
	// JoinHorizontal(infoView, helpView) header in frame() doesn't exceed the
	// terminal and soft-wrap. The extra -1 dodges a bubbles/help edge case:
	// when columns sum to exactly m.help.Width, the next column still
	// triggers truncation (>) but the ellipsis-fits check (<) fails, so the
	// bubble falls through and renders the column uncondensed.
	m.help.Width = max(w-lipgloss.Width(m.infoView())-1, 0)
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
	} else {
		table.WithWidth(w - borderWidth)(m.table)
		// updateColumns before WithHeight: WithHeight derives viewport.Height
		// from lipgloss.Height(headersView()), so columns must be set first
		// (otherwise an empty header is measured as 1 line and the viewport
		// ends up 1 line too tall once columns are populated — the top header
		// row gets clipped by bubbletea's renderer until a manual resize).
		m.updateColumns()
		table.WithHeight(h - borderHeight - top)(m.table)
		m.table.UpdateViewport()
	}
}

// refreshDisplay resets filters and updates columns and rows.
func (m *Model) refreshDisplay() {
	m.curFilter = ""
	m.textInput.Reset()
	m.updateColumns()
	m.updateRows(true)
}

// processData updates the model's dataset based on the incoming DataMsg.
//
//nolint:cyclop // legacy DataMsg router (see typed handlers below); complexity comes from the message type-switch.
func (m *Model) processData(msg DataMsg) tea.Cmd {
	var cmd tea.Cmd
	// Drop stale responses based on generation token (allow zero-value Gen).
	// Still endTask: the matching beginTask was already issued when the
	// load started, so the task must end to keep pendingTasks balanced.
	// Without this, a stale drop leaves the model permanently in
	// LoadingView — startup hang regression on `toolkit -c <lazy-cat>`.
	if msg.Gen != 0 && msg.Gen != m.gen {
		m.endTask(true)
		return nil
	}
	switch data := msg.Data.(type) {
	case *models.Dataset:
		m.dataset = data
	case []models.BaseModel:
		m.dataset.BaseModels = data
	case map[string][]models.ImportedModel:
		m.dataset.SetImportedModelMap(data)
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

/*
applyDataset standardizes how dataset mutations are applied and the UI is refreshed.
It:
- Ensures m.dataset is non-nil
- Applies the provided mutation
- Calls endTask(true)
- Logs a consistent "data loaded" message with category and count
- Refreshes the display
*/
func (m *Model) applyDataset(mut func(*models.Dataset), category domain.Category, count int) {
	if m.dataset == nil {
		m.dataset = &models.Dataset{}
	}
	mut(m.dataset)
	m.endTask(true)
	m.logger.Infow("data loaded", "category", category, "count", count, "pendingTasks", m.pendingTasks)
	m.refreshDisplay()
}

// Typed lazy-load handlers (replace DataMsg type-switch path)
// Each handler updates the dataset, ends the task, logs, refreshes display,
// and returns any follow-up command (e.g., GPU pool state enrichment).
// Each typed loaded-handler gates on gen to drop stale responses
// from superseded loads. Every drop path still calls endTask: the
// matching beginTask was issued when the load started, so the task
// must end to keep pendingTasks balanced — otherwise a stale drop
// strands the model in LoadingView (startup hang on
// `toolkit -c <lazy-cat>`, regression locked by
// TestStartupHang_LazyCategory).

func (m *Model) handleBaseModelsLoaded(items []models.BaseModel, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	m.applyDataset(func(ds *models.Dataset) { ds.BaseModels = items }, domain.BaseModel, len(items))
}

func (m *Model) handleImportedModelsLoaded(items map[string][]models.ImportedModel, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	total := 0
	for _, v := range items {
		total += len(v)
	}
	m.applyDataset(func(ds *models.Dataset) { ds.SetImportedModelMap(items) }, domain.ImportedModel, total)
}

func (m *Model) handleGpuPoolsLoaded(items []models.GpuPool, gen int) tea.Cmd {
	if gen != m.gen {
		m.endTask(true)
		return nil
	}
	m.applyDataset(func(ds *models.Dataset) { ds.GpuPools = items }, domain.GpuPool, len(items))
	return m.updateGpuPoolState()
}

func (m *Model) handleGpuNodesLoaded(items map[string][]models.GpuNode, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	total := 0
	for _, v := range items {
		total += len(v)
	}
	m.applyDataset(func(ds *models.Dataset) { ds.GpuNodeMap = items }, domain.GpuNode, total)
}

func (m *Model) handleDedicatedAIClustersLoaded(items map[string][]models.DedicatedAICluster, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	total := 0
	for _, v := range items {
		total += len(v)
	}
	m.applyDataset(func(ds *models.Dataset) { ds.SetDedicatedAIClusterMap(items) }, domain.DedicatedAICluster, total)
}

func (m *Model) handleTenancyOverridesLoaded(group models.TenancyOverrideGroup, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	m.applyDataset(func(ds *models.Dataset) {
		ds.Tenants = group.Tenants
		ds.LimitTenancyOverrideMap = group.LimitTenancyOverrideMap
		ds.ConsolePropertyTenancyOverrideMap = group.ConsolePropertyTenancyOverrideMap
		ds.PropertyTenancyOverrideMap = group.PropertyTenancyOverrideMap
	}, domain.Tenant, len(group.Tenants))
}

func (m *Model) handleLimitRegionalOverridesLoaded(items []models.LimitRegionalOverride, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	m.applyDataset(func(ds *models.Dataset) { ds.LimitRegionalOverrides = items }, domain.LimitRegionalOverride, len(items))
}

func (m *Model) handleConsolePropertyRegionalOverridesLoaded(items []models.ConsolePropertyRegionalOverride, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	m.applyDataset(func(ds *models.Dataset) { ds.ConsolePropertyRegionalOverrides = items }, domain.ConsolePropertyRegionalOverride, len(items))
}

func (m *Model) handlePropertyRegionalOverridesLoaded(items []models.PropertyRegionalOverride, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	m.applyDataset(func(ds *models.Dataset) { ds.PropertyRegionalOverrides = items }, domain.PropertyRegionalOverride, len(items))
}

func (m *Model) sortTableByColumn(column string) tea.Cmd {
	if m.sortColumn == column {
		m.sortAsc = !m.sortAsc
	} else {
		m.sortColumn = column
		m.sortAsc = true
	}

	m.updateColumns()
	return m.updateRowsAsync()
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
