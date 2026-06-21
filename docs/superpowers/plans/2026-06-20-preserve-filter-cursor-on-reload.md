# Preserve filter + cursor across data-load refreshes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the active filter and the user's selected row intact when the data behind the current category reloads (live-watch tick, manual refresh, post-edit), clearing them only on category navigation.

**Architecture:** Move the "clear filter" side effect out of the shared load-time `refreshDisplay()` and into the navigation path (`updateCategoryCore`, category-changed branch). Make `applyRows` re-home the cursor onto the previously-selected row by its Name cell, falling back to the existing scope/environment logic when there is no prior selection (which is the case after a navigation blanks the table).

**Tech Stack:** Go, Bubble Tea TUI (`internal/ui/tui`), `charmbracelet/bubbles/table`.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-06-20-preserve-filter-cursor-on-reload-design.md`
- Test command for the package: `go test ./internal/ui/tui/...`
- Follow existing TUI patterns; no new model fields — reuse `selectedRawRow()`, `findContextIndex`, `rawRows`.
- Run all commands from the repo root `/Users/jinguzha/Work/repos/toolkit`.

---

### Task 1: Navigation clears the filter

Move filter-clear to the category-changed branch of `updateCategoryCore`. This must land before Task 2 removes the redundant clear in `refreshDisplay`, so the filter is never silently left un-cleared on navigation.

**Files:**
- Modify: `internal/ui/tui/reducer_category.go:37-51` (the `else` branch where `m.category != category`)
- Test: `internal/ui/tui/reducer_category_watch_test.go` (add one test)

**Interfaces:**
- Consumes: `(m *Model) updateCategoryCore(category domain.Category) []tea.Cmd`, fields `m.filter string`, `m.textInput` (has `.Reset()`).
- Produces: nothing new; navigation now guarantees `m.filter == ""` and a reset input after a category change.

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/tui/reducer_category_watch_test.go`:

```go
// TestUpdateCategoryCore_NavigationClearsFilter asserts that navigating to a
// different category resets the active filter and the filter input.
func TestUpdateCategoryCore_NavigationClearsFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.Environment
	m.filter = "stale"
	m.textInput.SetValue("stale")

	_ = m.updateCategoryCore(domain.GPUNode)

	if m.filter != "" {
		t.Fatalf("navigation did not clear filter: %q", m.filter)
	}
	if m.textInput.Value() != "" {
		t.Fatalf("navigation did not reset input: %q", m.textInput.Value())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestUpdateCategoryCore_NavigationClearsFilter -v`
Expected: FAIL — `navigation did not clear filter: "stale"` (the filter is only cleared later, at load time).

- [ ] **Step 3: Add the filter reset to the category-changed branch**

In `internal/ui/tui/reducer_category.go`, inside the `else` block that runs when `m.category != category` (currently lines 37-51), add the two reset lines alongside the existing view-state resets. After the edit the branch reads:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestUpdateCategoryCore_NavigationClearsFilter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/reducer_category.go internal/ui/tui/reducer_category_watch_test.go
git commit -m "feat(tui): clear filter on category navigation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `refreshDisplay` preserves the filter

Remove the filter-clear from the shared load-time refresh. Invert the existing test that asserted the opposite.

**Files:**
- Modify: `internal/ui/tui/model_reducer.go:296-302` (`refreshDisplay`)
- Test: `internal/ui/tui/model_reducer_test.go:81-96` (invert + rename `TestApplyDataset_CurrentCategoryLoadRefreshes`)

**Interfaces:**
- Consumes: `(m *Model) refreshDisplay()`, `(m *Model) applyDataset(...)`.
- Produces: `refreshDisplay()` no longer mutates `m.filter` / `m.textInput`; a same-category data load preserves the active filter.

- [ ] **Step 1: Invert the existing test (make it the failing test)**

In `internal/ui/tui/model_reducer_test.go`, replace the existing `TestApplyDataset_CurrentCategoryLoadRefreshes` (lines 81-96, including its leading comment) with:

```go
// A load for the category currently on screen refreshes the rows but PRESERVES
// the active filter — only category navigation clears it (see
// TestUpdateCategoryCore_NavigationClearsFilter).
func TestApplyDataset_CurrentCategoryLoadPreservesFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.filter = "keep"

	m.applyDataset(func(ds *models.Dataset) {
		ds.BaseModels = []models.BaseModel{{Name: "bm1"}}
	}, domain.BaseModel, 1)

	if m.filter != "keep" {
		t.Fatalf("current-category load cleared the filter: %q", m.filter)
	}
	if len(m.dataset.BaseModels) != 1 {
		t.Fatalf("current-category load did not apply data: %#v", m.dataset.BaseModels)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestApplyDataset_CurrentCategoryLoadPreservesFilter -v`
Expected: FAIL — `current-category load cleared the filter: ""` (refreshDisplay still resets it).

- [ ] **Step 3: Remove the filter-clear from `refreshDisplay`**

In `internal/ui/tui/model_reducer.go`, replace `refreshDisplay` (lines 296-302) with:

```go
// refreshDisplay re-renders columns and rows for the current category,
// preserving the active filter and the user's selected row. The filter is
// cleared only on category navigation (updateCategoryCore).
func (m *Model) refreshDisplay() {
	m.updateColumns()
	m.updateRows(true)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestApplyDataset_CurrentCategoryLoadPreservesFilter -v`
Expected: PASS

- [ ] **Step 5: Run the full package to catch fallout**

Run: `go test ./internal/ui/tui/...`
Expected: PASS. If a test fails because it assumed a same-category reload clears the filter, that assumption is now wrong per the spec — update that test to expect preservation. (Note any such edits in the commit message.)

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/model_reducer.go internal/ui/tui/model_reducer_test.go
git commit -m "feat(tui): preserve filter across same-category data refresh

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `applyRows` preserves the selected row across reloads

Re-home the cursor onto the previously-selected row by Name, falling back to the existing scope/environment logic.

**Files:**
- Modify: `internal/ui/tui/model_reducer.go:95-117` (`applyRows`)
- Add: `indexOfRow` helper in `internal/ui/tui/table_utils.go` (next to `cloneRows`/`selectedRawRow`)
- Test: `internal/ui/tui/model_reducer_test.go` (add two tests)

**Interfaces:**
- Consumes: `(m *Model) selectedRawRow() table.Row`, `(m *Model) findContextIndex(rows []table.Row) int`, `m.table` (`Cursor()`, `Rows()`, `GotoTop()`, `MoveDown(int)`, `SetCursor(int)`).
- Produces: `func indexOfRow(rows []table.Row, name string) int` — index of the first row whose cell `[0]` equals `name`, or `-1`. `applyRows` keeps the cursor on the prior selection when that row still exists.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/tui/model_reducer_test.go`:

```go
// A same-category reload keeps the cursor on the row the user had selected,
// matched by its Name cell, even when its index shifts.
func TestApplyRows_PreservesSelectedRowAcrossReload(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()   // populate columns + rows
	m.table.SetCursor(1) // select bm2

	m.refreshDisplay() // simulate an in-place reload of the same category

	got := m.selectedRawRow()
	if len(got) == 0 || got[0] != "bm2" {
		t.Fatalf("cursor not preserved on reload: %v", got)
	}
}

// When the previously-selected row is gone after a reload, the cursor clamps to
// a valid index instead of pointing past the end.
func TestApplyRows_ClampsWhenSelectedRowDisappears(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()
	m.table.SetCursor(1) // select bm2

	m.dataset.BaseModels = []models.BaseModel{{Name: "bm1"}, {Name: "bm3"}}
	m.refreshDisplay() // bm2 no longer present

	c := m.table.Cursor()
	if c < 0 || c >= len(m.table.Rows()) {
		t.Fatalf("cursor out of range after reload: %d (rows=%d)", c, len(m.table.Rows()))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/tui/ -run 'TestApplyRows_PreservesSelectedRowAcrossReload|TestApplyRows_ClampsWhenSelectedRowDisappears' -v`
Expected: `TestApplyRows_PreservesSelectedRowAcrossReload` FAILS — cursor falls back to top (bm1) because `applyRows` only restores the scope/env row, not the user's selection. (The clamp test may already pass, since today's fallback is `GotoTop`; it guards against regressions from Step 3.)

- [ ] **Step 3: Add the `indexOfRow` helper**

In `internal/ui/tui/table_utils.go`, immediately after `selectedRawRow` (ends at line 260), add:

```go
// indexOfRow returns the index of the first row whose Name cell (column 0)
// equals name, or -1 when name is empty or no row matches.
func indexOfRow(rows []table.Row, name string) int {
	if name == "" {
		return -1
	}
	for i, r := range rows {
		if len(r) > 0 && r[0] == name {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 4: Make `applyRows` preserve the selection**

In `internal/ui/tui/model_reducer.go`, replace `applyRows` (lines 95-117) with:

```go
func (m *Model) applyRows(rows []table.Row, stats tableStats, autoSelect bool) {
	// Capture the prior selection before m.rawRows is replaced below, so an
	// in-place reload can re-home the cursor onto the same row by identity.
	// After a navigation the table was blanked (applyRows(nil, ..., false)),
	// so there is no prior selection and prevName stays "" — the cursor then
	// falls through to findContextIndex (scope/environment), preserving the
	// pre-existing navigation behavior.
	var prevName string
	if autoSelect {
		if r := m.selectedRawRow(); len(r) > 0 {
			prevName = r[0]
		}
	}

	m.stats = stats
	m.rawRows = cloneRows(rows)
	m.applyMiddleTruncation(rows)
	table.WithRows(rows)(m.table)

	if autoSelect {
		idx := indexOfRow(rows, prevName)
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
```

- [ ] **Step 5: Run the new tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'TestApplyRows_PreservesSelectedRowAcrossReload|TestApplyRows_ClampsWhenSelectedRowDisappears' -v`
Expected: PASS (both).

- [ ] **Step 6: Run the full package**

Run: `go test ./internal/ui/tui/...`
Expected: PASS. A pre-existing test that asserted a same-category reload jumps the cursor to top/scope is now outdated — update it to expect the selection is preserved, and note the edit in the commit.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/tui/model_reducer.go internal/ui/tui/table_utils.go internal/ui/tui/model_reducer_test.go
git commit -m "feat(tui): keep selected row across same-category reload

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Live-watch reload end-to-end guard

Lock the user-reported behavior — a live-watch tick preserves both filter and selection — with a test that drives the watch-trigger path.

**Files:**
- Test: `internal/ui/tui/reducer_watch_test.go` (add one test)

**Interfaces:**
- Consumes: `(m *Model) handleWatchTriggered(msg watchTriggeredMsg) tea.Cmd`, the typed loaded-handler `(m *Model) handleBaseModelsLoaded(items []models.BaseModel, gen int)`, and `m.gen`.

- [ ] **Step 1: Confirm the watch-trigger test fixtures**

Read `internal/ui/tui/reducer_watch_test.go` around the existing `TestHandleWatchTriggered_*` tests to copy the exact `watchTriggeredMsg` construction (field names `Cat`, `Trigger`, `Gen`) and any helper used to arm `m.watchTrigger`.

Run: `go test ./internal/ui/tui/ -run TestHandleWatchTriggered -v`
Expected: PASS (baseline — the existing watch tests still pass).

- [ ] **Step 2: Write the test**

Add to `internal/ui/tui/reducer_watch_test.go` (adjust the `watchTriggeredMsg` literal to match the field names confirmed in Step 1):

```go
// A live-watch reload of the on-screen category preserves the active filter and
// the selected row — it does not behave like a navigation.
func TestLiveReload_PreservesFilterAndSelection(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()
	m.table.SetCursor(1) // select bm2
	m.filter = "keep"

	// Simulate the data landing from a watch-triggered reload (same gen, same
	// category) via the typed loaded-handler the reload command resolves to.
	m.handleBaseModelsLoaded([]models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}, 1)

	if m.filter != "keep" {
		t.Fatalf("live reload cleared the filter: %q", m.filter)
	}
	if got := m.selectedRawRow(); len(got) == 0 || got[0] != "bm2" {
		t.Fatalf("live reload lost the selection: %v", got)
	}
}
```

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestLiveReload_PreservesFilterAndSelection -v`
Expected: PASS (Tasks 1-3 already implement the behavior; this test guards the end-to-end path).

- [ ] **Step 4: Run the full package and the linter**

Run: `go test ./internal/ui/tui/...`
Expected: PASS

Run: `golangci-lint run ./internal/ui/tui/...` (or the repo's configured lint command)
Expected: no new findings.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/reducer_watch_test.go
git commit -m "test(tui): guard live-reload filter+selection preservation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Change 1 (navigation clears filter) → Task 1. ✓
- Change 2 (`refreshDisplay` stops clearing filter) → Task 2. ✓
- Change 3 (`applyRows` preserves selected row, clamps) → Task 3. ✓
- Testing section: inverted existing test → Task 2 Step 1; live-reload filter+cursor → Task 4; clamp → Task 3; navigation clears → Task 1. ✓
- Consequence "all loads preserve, nav clears" → covered by Tasks 1-2 plus the full-package runs in Task 2/3 that update any outdated assumptions. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code; test bodies are concrete. Task 4 Step 1 instructs reading the fixture for exact field names — this is a verification step, not a placeholder, because `watchTriggeredMsg`'s constructor is in a file not fully quoted here. ✓

**Type consistency:** `indexOfRow(rows []table.Row, name string) int` defined in Task 3 Step 3, used in Task 3 Step 4. `selectedRawRow() table.Row` and `findContextIndex(rows) int` used as they exist today. `handleBaseModelsLoaded(items []models.BaseModel, gen int)` matches `model_reducer.go:388`. Field `m.filter` / `m.textInput` match `model_state.go`. ✓
