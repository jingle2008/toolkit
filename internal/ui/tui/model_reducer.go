// Package tui contains reducer and event logic for the Model.
// This file contains methods for state transitions, event handling, and UI updates.
package tui

import (
	"context"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

func refreshDataCmd() tea.Cmd { return func() tea.Msg { return dataMsg{} } }

// knownCompartmentID returns the compartment of any loaded GPU node, or
// "" when none is loaded.
func (m *Model) knownCompartmentID() string {
	if m.dataset == nil {
		return ""
	}
	for _, v := range m.dataset.GPUNodeMap {
		for _, n := range v {
			return n.CompartmentID
		}
	}
	return ""
}

// compartmentIDFn is the seam tests use to fake the cluster lookup.
var compartmentIDFn = resolve.CompartmentID

// lookupCompartmentID returns known, or else env's cluster compartment as
// resolved (and cached) by resolve.CompartmentID, the lookup the CLI and
// MCP use; a cluster without GPU nodes is an error. It reads no Model
// state, so it is safe off the Update loop.
func lookupCompartmentID(ctx context.Context, known, kubeConfig string, env models.Environment) (string, error) {
	if known != "" {
		return known, nil
	}
	return compartmentIDFn(ctx, kubeConfig, env)
}

// updateGPUPoolState enriches the GPU pools with their OCI instance-pool
// state (ID, actual size, lifecycle). It works on a snapshot taken here, on
// the Update loop, and handleUpdateDoneMsg merges the result back: writing
// the live pool list from the background job would race the Update loop,
// and the job may finish after an environment switch.
func (m *Model) updateGPUPoolState() tea.Cmd {
	env, kubeConfig, known := m.environment, m.kubeConfig, m.knownCompartmentID()
	var pools []models.GPUPool
	if m.dataset != nil {
		pools = slices.Clone(m.dataset.GPUPools)
	}
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		compartmentID, err := lookupCompartmentID(ctx, known, kubeConfig, env)
		if err == nil {
			err = oci.PopulateGPUPools(ctx, pools, env, compartmentID)
		}
		return updateDoneMsg{err: err, category: domain.GPUPool, env: env, pools: pools}
	}
}

/*
updateRows updates the table rows based on the current model state.
Now also sets m.stats from computeTableRows.
*/
func (m *Model) updateRows(autoSelect bool) {
	m.gens.nextRows()
	rows, stats := computeTableRows(m.dataset, m.category, m.scope, m.filter, m.sortColumn, m.sortAsc, m.showFaulty)
	total := rowTotal(m.dataset, m.category, m.scope, m.filter, m.showFaulty, len(rows))
	m.applyRows(rows, stats, autoSelect)
	m.rowTotal = total
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
			Total: rowTotal(dataset, category, scope, filter, showFaulty, len(rows)),
			Gen:   gen,
		}
	}
}

func (m *Model) handleTableRowsComputedMsg(msg tableRowsComputedMsg) {
	if msg.Gen != m.gens.rows {
		return
	}
	m.applyRows(msg.Rows, msg.Stats, true)
	m.rowTotal = msg.Total
}

func (m *Model) applyRows(rows []table.Row, stats tableStats, autoSelect bool) {
	m.detailStale = true
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
	m.rowTotal = len(rows) // updateRows and the async path set the true total after
	m.rawRows = cloneRows(rows)
	m.updateColumns() // widths follow the values
	m.decorateRows(rows)
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

// decorateRows turns rows, plain cells as in m.rawRows, into display
// rows for the current columns: running actions' statuses, cells cut to
// their width in the middle, status colors, muted IDs, empty-cell dashes
// and right-aligned numbers.
func (m *Model) decorateRows(rows []table.Row) {
	m.overlayActionStatus(rows)
	m.applyMiddleTruncation(rows)
	m.toneStatusCells(rows)
	m.muteIDCells(rows)
	m.markEmptyCells(rows)
	m.alignNumericCells(rows)
}

// redecorateRows redraws the table's rows from m.rawRows after the column
// widths change without new rows (a resize), so middle-truncated cells
// are cut to the new widths rather than end-truncated by the table.
func (m *Model) redecorateRows() {
	if len(m.rawRows) == 0 || len(m.rawRows[0]) != len(m.headers) {
		return // nothing shown, or rows left from another category
	}
	rows := cloneRows(m.rawRows)
	m.decorateRows(rows)
	table.WithRows(rows)(m.table)
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

// updateColumns sets the table's columns for the current category:
// titles (the sort column's arrow, and each sortable column's sort key
// underlined, so the key is there to read before s is pressed) and widths fitted to the titles and the rows'
// values by columnWidths. Columns that don't fit are hidden and named
// in m.hiddenColumns.
func (m *Model) updateColumns() {
	m.headers = headersFor(m.category)
	sortKeys := map[string]string{} // lowercased column -> sort key
	for col, k := range keys.SortKeys(m.category) {
		sortKeys[strings.ToLower(col)] = k
	}
	titles := make([]string, len(m.headers))
	specs := make([]columnSpec, len(m.headers))
	content := m.contentWidths()
	for i, header := range m.headers {
		title := header.text
		if k, ok := sortKeys[strings.ToLower(header.text)]; ok {
			title = underlineSortKey(title, k)
		}
		if m.sortColumn != "" && strings.EqualFold(header.text, m.sortColumn) {
			if m.sortAsc {
				title += " ↑"
			} else {
				title += " ↓"
			}
		}
		titles[i] = title
		status := strings.EqualFold(header.text, common.StatusCol)
		specs[i] = columnSpec{
			title:   lipgloss.Width(title),
			content: content[i],
			ratio:   header.ratio,
			keep:    i == 0 || status,
			whole:   status,
		}
	}
	widths := columnWidths(specs, m.table.Width(), m.styles.Header.GetHorizontalFrameSize())
	for i, header := range m.headers {
		if isNumericCol(header.text) {
			titles[i] = padLeft(titles[i], widths[i])
		}
	}
	columns := make([]table.Column, len(m.headers))
	m.hiddenColumns = nil
	for i, header := range m.headers {
		columns[i] = table.Column{Title: titles[i], Width: widths[i]}
		if widths[i] == 0 {
			m.hiddenColumns = append(m.hiddenColumns, header.text)
		}
	}
	table.WithColumns(columns)(m.table)
}

// contentWidths is each column's widest value in m.rawRows, an empty cell
// counting as its "—". Rows left from another category (their cell count
// differs) count as none.
func (m *Model) contentWidths() []int {
	widths := make([]int, len(m.headers))
	for _, r := range m.rawRows {
		if len(r) != len(widths) {
			continue
		}
		for i, cell := range r {
			widths[i] = max(widths[i], runewidth.StringWidth(cell), 1)
		}
	}
	return widths
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
	case m.landOn != nil && m.category == m.landOn.Category:
		name = m.landOn.Name
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
	// The export modal shows four lines above the directory list; give the
	// list the rest of the modal body, up to its usual 15 rows.
	if m.dirPicker != nil {
		m.dirPicker.SetHeight(min(exportListMax, max(m.modalMaxBody()-exportChrome, 3)))
	}
	// The frame draws its panel's border itself: two columns and two rows
	// around the table or details body, plus frameChrome rows of bars.
	inner, body := max(w-2, 0), max(h-frameChrome-2, 1)
	if m.viewMode == common.DetailsView {
		m.viewport.SetWidth(inner)
		m.viewport.SetHeight(body)
	} else {
		tableW := inner
		if m.previewShown() {
			tableW = w - m.previewOuterWidth() - 2
		}
		table.WithWidth(tableW)(m.table)
		// updateColumns before WithHeight: WithHeight derives viewport.Height
		// from lipgloss.Height(headersView()), so columns must be set first
		// (otherwise an empty header is measured as 1 line and the viewport
		// ends up 1 line too tall once columns are populated — the top header
		// row gets clipped by bubbletea's renderer until a manual resize).
		m.updateColumns()
		m.redecorateRows()
		table.WithHeight(body)(m.table)
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
		m.mergeRepoDataset(ds)
	}
	if msg.Data != nil {
		m.endTask(true)
		m.logger.Infow("data loaded", "category", m.category, "pendingTasks", m.pendingTasks)
	}
	m.refreshDisplay()
}

/*
mergeRepoDataset takes in a freshly loaded repo dataset (LoadDataset: the
repo-owned fields). A lazily loaded category can land first — `-c gpup`
whose pools beat the startup load, or a repo reload with cluster
categories cached — so the cluster fields already held are kept rather
than wiped, and their owners re-resolved against the fresh tenants (a
tenant renamed in the metadata, or tenants now known).
*/
func (m *Model) mergeRepoDataset(fresh *models.Dataset) {
	if m.dataset == nil {
		m.dataset = fresh
		return
	}
	m.dataset.MergeReloadedRepoData(fresh)
	resolveTenantOwners(m.dataset)
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

func (m *Model) handleServingRuntimesLoaded(items []models.ServingRuntime, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.ServingRuntimes = items }, domain.ServingRuntime, len(items))
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

func (m *Model) handleInferenceServicesLoaded(items map[string][]models.InferenceService, gen int) {
	m.applyLoaded(gen, func(ds *models.Dataset) { ds.SetInferenceServiceMap(items) }, domain.InferenceService, mapLen(items))
}

func (m *Model) handleTenancyOverridesLoaded(group models.TenancyOverrideGroup, gen int) {
	// The group fills four categories and names the owners of four more;
	// name the one on screen, if any, so applyDataset redraws it (an
	// environment switch in place on LimitTenancyOverride clears its rows
	// and relies on this load; one on DACs shows raw tenancy IDs until it).
	cat := domain.Tenant
	_, override := tenancyOverrideCategories[m.category]
	_, owned := tenantOwnedCategories[m.category]
	if override || owned {
		cat = m.category
	}
	m.applyLoaded(gen, func(ds *models.Dataset) {
		ds.Tenants = group.Tenants
		ds.LimitTenancyOverrideMap = group.LimitTenancyOverrideMap
		ds.ConsolePropertyTenancyOverrideMap = group.ConsolePropertyTenancyOverrideMap
		ds.PropertyTenancyOverrideMap = group.PropertyTenancyOverrideMap
		// Cluster categories may have loaded first (e.g. an environment
		// switch while viewing DACs): resolve their owners against the
		// new tenants, or drilling in from a tenant would miss them.
		resolveTenantOwners(ds)
	}, cat, len(group.Tenants))
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
	case domain.ServingRuntime:
		return loadServingRuntimesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUNode:
		return loadGPUNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUWorkload:
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.DedicatedAICluster:
		return loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.InferenceService:
		return loadInferenceServicesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
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
	if msg.Gen != m.gens.msg || msg.Cat != m.category {
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
	// Leaving a cluster category for one without a loader (a definition,
	// the alias list) cancels its watch without bumping the gen: its
	// close is then still current, but for a category no longer shown,
	// and must not reload it in the background.
	if msg.Gen != m.gens.msg || msg.Cat != m.category {
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
