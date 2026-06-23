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

func refreshDataCmd() tea.Cmd { return func() tea.Msg { return dataMsg{} } }

func (m *Model) lookupCompartmentID(ctx context.Context) (string, error) {
	if m.dataset != nil && m.dataset.GPUNodeMap != nil {
		for _, v := range m.dataset.GPUNodeMap {
			for _, n := range v {
				return n.CompartmentID, nil
			}
		}
	}

	clientset, err := k8s.NewClientsetFromKubeConfig(m.kubeConfig, m.environment.KubeContext())
	if err != nil {
		return "", err
	}
	nodes, err := k8s.ListGPUNodes(ctx, clientset, 1)
	if err != nil || len(nodes) == 0 {
		return "", err
	}

	return nodes[0].CompartmentID, nil
}

func (m *Model) updateGPUPoolState() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		var err error
		var compartmentID string
		if compartmentID, err = m.lookupCompartmentID(ctx); err == nil {
			err = actions.PopulateGPUPools(ctx, m.dataset.GPUPools, m.environment, compartmentID)
		}
		return updateDoneMsg{err: err, category: domain.GPUPool}
	}
}

/*
updateRows updates the table rows based on the current model state.
Now also sets m.stats from computeTableRows.
*/
func (m *Model) updateRows(autoSelect bool) {
	m.gens.nextRows()
	rows, stats := computeTableRows(m.dataset, m.category, m.scope, m.filter, m.sortColumn, m.sortAsc, m.showFaulty)
	m.applyRows(rows, stats, autoSelect)
}

func (m *Model) updateRowsAsync() tea.Cmd {
	gen := m.gens.nextRows()
	dataset := m.dataset
	category := m.category
	scope := m.scope
	filter := m.filter
	sortColumn := m.sortColumn
	sortAsc := m.sortAsc
	showFaulty := m.showFaulty
	return func() tea.Msg {
		rows, stats := computeTableRows(dataset, category, scope, filter, sortColumn, sortAsc, showFaulty)
		return tableRowsComputedMsg{
			Rows:  rows,
			Stats: stats,
			Gen:   gen,
		}
	}
}

func (m *Model) handleTableRowsComputedMsg(msg tableRowsComputedMsg) {
	if msg.Gen != m.gens.rows {
		return
	}
	m.applyRows(msg.Rows, msg.Stats, true)
}

func (m *Model) applyRows(rows []table.Row, stats tableStats, autoSelect bool) {
	// Capture the prior selection before m.rawRows is replaced below, so an
	// in-place reload can re-home the cursor onto the same item by identity.
	// Identity is the per-category item key (itemKeyFrom), not the bare Name
	// cell, so scoped categories whose rows can share a Name (e.g.
	// ImportedModel keyed on {Scope, Name}) re-home onto the right row.
	// prevIdx is the selection's offset, used for the fast path below.
	// After a navigation the table was blanked (applyRows(nil, ..., false)),
	// so there is no prior selection and prevKey stays nil — the cursor then
	// falls through to findContextIndex (scope/environment), preserving the
	// pre-existing navigation behavior.
	var prevKey models.ItemKey
	prevIdx := -1
	if autoSelect {
		prevIdx = m.table.Cursor()
		prevKey = itemKeyFrom(m.category, m.selectedRawRow())
	}

	m.stats = stats
	m.rawRows = cloneRows(rows)
	m.applyMiddleTruncation(rows)
	table.WithRows(rows)(m.table)

	if autoSelect {
		// Match identity against m.rawRows, not the just-truncated `rows`:
		// applyMiddleTruncation may have shortened the key cells (Name/Tenant
		// for scoped categories), which would defeat the key comparison.
		idx := -1
		// Fast path: if the selected item still sits at its previous offset,
		// skip the scan. Reloads usually preserve order, so this hits often.
		if prevKey != nil && prevIdx >= 0 && prevIdx < len(m.rawRows) &&
			itemKeyFrom(m.category, m.rawRows[prevIdx]) == prevKey {
			idx = prevIdx
		}
		if idx < 0 {
			idx = indexOfItemKey(m.rawRows, m.category, prevKey)
		}
		if idx < 0 {
			idx = m.findContextIndex(rows)
		}
		if idx >= 0 {
			// SetCursor moves the cursor and render window but leaves the
			// viewport's scroll offset untouched, so a target beyond the
			// first page lands one row below the visible window (off by
			// one). GotoTop then MoveDown drives bubbles' own scroll logic,
			// which brings the row fully into view.
			m.table.GotoTop()
			m.table.MoveDown(idx)
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
	m.headers = headersFor(m.category)
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
	case m.scope != nil && m.category == m.scope.Category:
		name = m.scope.Name
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
		borderStyle = m.theme.Base.GetBorderStyle()
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

// refreshDisplay re-renders columns and rows for the current category,
// preserving the active filter and the user's selected row. The filter is
// cleared only on category navigation (updateCategoryCore).
func (m *Model) refreshDisplay() {
	m.updateColumns()
	m.updateRows(true)
}

// handleDataMsg updates the model's dataset based on the incoming dataMsg.
func (m *Model) handleDataMsg(msg dataMsg) {
	// Drop stale responses based on generation token (allow zero-value Gen).
	// Still endTask: the matching beginTask was already issued when the
	// load started, so the task must end to keep pendingTasks balanced.
	// Without this, a stale drop leaves the model permanently in
	// LoadingView — startup hang regression on `toolkit -c <lazy-cat>`.
	if msg.Gen != 0 && msg.Gen != m.gens.msg {
		m.endTask(true)
		return
	}
	// dataMsg has two live roles: the foundational dataset load
	// (*models.Dataset) and a nil-payload refresh signal (refreshDataCmd).
	// Per-category data is owned solely by the typed *LoadedMsg handlers.
	if ds, ok := msg.Data.(*models.Dataset); ok {
		m.dataset = ds
	}
	if msg.Data != nil {
		m.endTask(true)
		m.logger.Infow("data loaded", "category", m.category, "pendingTasks", m.pendingTasks)
	}
	m.refreshDisplay()
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
	// Only rebuild the visible table when the load is for the category
	// currently on screen. A background load for another category — e.g. a
	// model catalog fetched to resolve a DAC's metrics capability while the
	// DAC list is showing — must not refresh the current table: it would
	// recompute rows for the wrong category and disturb the visible cursor.
	// The data is still cached on the dataset above, so it's ready when the
	// user later navigates to that category. Every genuine navigation load
	// sets m.category to the destination before dispatching
	// (updateCategoryCore), so this guard is a no-op for them.
	if category == m.category {
		m.refreshDisplay()
	}
}

// Typed lazy-load handlers (replace dataMsg type-switch path)
// Each handler updates the dataset, ends the task, logs, refreshes display,
// and returns any follow-up command (e.g., GPU pool state enrichment).
// Each typed loaded-handler gates on gen to drop stale responses
// from superseded loads. Every drop path still calls endTask: the
// matching beginTask was issued when the load started, so the task
// must end to keep pendingTasks balanced — otherwise a stale drop
// strands the model in LoadingView (startup hang on
// `toolkit -c <lazy-cat>`, regression locked by
// TestStartupHang_LazyCategory).

// applyLoaded drops a stale load — one whose captured gen no longer
// matches the current generation — and otherwise applies the dataset
// mutation. The stale path still calls endTask to keep pendingTasks
// balanced (see the note above). Returns false when the load was stale.
func (m *Model) applyLoaded(gen int, mut func(*models.Dataset), cat domain.Category, count int) bool {
	if gen != m.gens.msg {
		m.endTask(true)
		return false
	}
	m.applyDataset(mut, cat, count)
	return true
}

// mapLen returns the total number of values across a map of slices.
func mapLen[K comparable, V any](m map[K][]V) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}

func (m *Model) handleBaseModelsLoaded(items []models.BaseModel, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.BaseModels = items }, domain.BaseModel, len(items))
}

func (m *Model) handleImportedModelsLoaded(items map[string][]models.ImportedModel, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.SetImportedModelMap(items) }, domain.ImportedModel, mapLen(items))
}

func (m *Model) handleGPUPoolsLoaded(items []models.GPUPool, gen int) tea.Cmd {
	if !m.applyLoaded(gen, func(ds *models.Dataset) { ds.GPUPools = items }, domain.GPUPool, len(items)) {
		return nil
	}
	return m.updateGPUPoolState()
}

func (m *Model) handleGPUNodesLoaded(items map[string][]models.GPUNode, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.GPUNodeMap = items }, domain.GPUNode, mapLen(items))
}

func (m *Model) handleGPUWorkloadsLoaded(items map[string][]models.GPUWorkload, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.SetGPUWorkloadMap(items) }, domain.GPUWorkload, mapLen(items))
}

func (m *Model) handleDedicatedAIClustersLoaded(items map[string][]models.DedicatedAICluster, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.SetDedicatedAIClusterMap(items) }, domain.DedicatedAICluster, mapLen(items))
}

func (m *Model) handleTenancyOverridesLoaded(group models.TenancyOverrideGroup, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) {
		ds.Tenants = group.Tenants
		ds.LimitTenancyOverrideMap = group.LimitTenancyOverrideMap
		ds.ConsolePropertyTenancyOverrideMap = group.ConsolePropertyTenancyOverrideMap
		ds.PropertyTenancyOverrideMap = group.PropertyTenancyOverrideMap
	}, domain.Tenant, len(group.Tenants))
}

func (m *Model) handleLimitRegionalOverridesLoaded(items []models.LimitRegionalOverride, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.LimitRegionalOverrides = items }, domain.LimitRegionalOverride, len(items))
}

func (m *Model) handleConsolePropertyRegionalOverridesLoaded(items []models.ConsolePropertyRegionalOverride, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.ConsolePropertyRegionalOverrides = items }, domain.ConsolePropertyRegionalOverride, len(items))
}

func (m *Model) handlePropertyRegionalOverridesLoaded(items []models.PropertyRegionalOverride, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.PropertyRegionalOverrides = items }, domain.PropertyRegionalOverride, len(items))
}

// reloadCategoryCmd returns the existing one-shot load command for a
// watched category. Used both for the trigger-driven refresh and the
// final reload when a watch dies. Returns nil for non-watched categories.
func (m *Model) reloadCategoryCmd(cat domain.Category, gen int) tea.Cmd {
	switch cat {
	case domain.BaseModel:
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUNode:
		return loadGPUNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUWorkload:
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.DedicatedAICluster:
		return loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	default:
		return nil
	}
}

// handleK8sWatchStarted marks the category live and arms the trigger
// listener. A stale gen (the user already navigated away) is ignored;
// the watch goroutine is already being torn down via loadCtx cancel.
func (m *Model) handleK8sWatchStarted(msg k8sWatchStartedMsg) tea.Cmd {
	if msg.Gen != m.gens.msg {
		m.logger.Debugw("watch started ignored (stale gen)", "category", msg.Cat, "msgGen", msg.Gen, "gen", m.gens.msg)
		return nil
	}
	m.watch.k8sActive = true
	m.watch.k8sTrigger = msg.Trigger
	m.logger.Infow("watch started", "category", msg.Cat, "gen", msg.Gen)
	return waitForK8sTriggerCmd(msg.Cat, msg.Trigger, msg.Gen)
}

// handleK8sWatchTriggered re-runs the category loader and re-arms the
// listener so subsequent changes keep flowing. Stale-generation
// messages (msg.Gen != m.gens.msg) are ignored without side effects.
func (m *Model) handleK8sWatchTriggered(msg k8sWatchTriggeredMsg) tea.Cmd {
	if msg.Gen != m.gens.msg {
		m.logger.Debugw("watch triggered ignored (stale gen)", "category", msg.Cat, "msgGen", msg.Gen, "gen", m.gens.msg)
		return nil
	}
	m.logger.Debugw("watch triggered", "category", msg.Cat, "gen", msg.Gen)
	reload := m.reloadCategoryCmd(msg.Cat, msg.Gen)
	if reload == nil {
		return nil
	}
	return tea.Batch(m.beginTask(), reload, m.waitForK8sTrigger(msg.Cat, msg.Gen))
}

// handleK8sWatchClosed falls back to a final one-shot load and clears the
// live indicator (no auto-reconnect).
func (m *Model) handleK8sWatchClosed(msg k8sWatchClosedMsg) tea.Cmd {
	if msg.Gen != m.gens.msg {
		m.logger.Debugw("watch closed ignored (stale gen)", "category", msg.Cat, "msgGen", msg.Gen, "gen", m.gens.msg)
		return nil
	}
	m.watch.k8sActive = false
	m.logger.Infow("watch closed — clearing live indicator (no reconnect)", "category", msg.Cat, "gen", msg.Gen)
	reload := m.reloadCategoryCmd(msg.Cat, msg.Gen)
	if reload == nil {
		return nil
	}
	return tea.Batch(m.beginTask(), reload)
}

// handleK8sWatchUnavailable records that no live watch is active. The
// static load result remains on screen.
func (m *Model) handleK8sWatchUnavailable(msg k8sWatchUnavailableMsg) {
	if msg.Gen != m.gens.msg {
		m.logger.Debugw("watch unavailable ignored (stale gen)", "category", msg.Cat, "msgGen", msg.Gen, "gen", m.gens.msg)
		return
	}
	m.watch.k8sActive = false
	m.logger.Infow("watch unavailable (no live watch)", "category", msg.Cat, "gen", msg.Gen)
}

// waitForK8sTrigger re-arms the listener on the stored trigger channel.
func (m *Model) waitForK8sTrigger(cat domain.Category, gen int) tea.Cmd {
	if m.watch.k8sTrigger == nil {
		return nil
	}
	return waitForK8sTriggerCmd(cat, m.watch.k8sTrigger, gen)
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
beginTask increments the pendingTasks counter and kicks off the
spinner + stopwatch ticks so the inline indicator in statusView can
animate. For the very first load (m.dataset == nil) we still swap
into the full-screen LoadingView because there's no content to layer
the indicator over; every subsequent load stays in the active view.
*/
func (m *Model) beginTask() tea.Cmd {
	var cmd tea.Cmd
	if m.pendingTasks == 0 {
		cmd = tea.Sequence(
			m.loadingSpinner.Tick,
			m.loadingTimer.Reset(),
			m.loadingTimer.Start(),
		)
		if m.dataset == nil {
			m.lastViewMode = m.viewMode
			m.viewMode = common.LoadingView
		}
	}
	m.pendingTasks++
	return cmd
}

/*
endTask decrements the pendingTasks counter. If we entered the
full-screen LoadingView for a first load, restore the prior view;
otherwise leave m.viewMode alone — the user may have navigated
(DetailsView, HelpView, …) while the load was in flight.
*/
func (m *Model) endTask(success bool) {
	if m.pendingTasks > 0 {
		m.pendingTasks--
	}
	if m.pendingTasks == 0 {
		elapsed := m.loadingTimer.Elapsed().String()
		m.logger.Infow(
			"data load completed",
			"category", m.category,
			"success", success,
			"elapsed", elapsed,
		)
		if m.viewMode == common.LoadingView {
			m.viewMode = m.lastViewMode
		}
	}
}
