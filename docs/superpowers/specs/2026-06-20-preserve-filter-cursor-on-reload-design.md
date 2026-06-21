# Preserve filter + cursor across data-load refreshes

**Date:** 2026-06-20
**Status:** Approved (design)

## Problem

Every data load in the TUI — the initial load, a live-watch reload, a manual
same-category refresh, and the post-edit refresh — converges on
`refreshDisplay()` (`internal/ui/tui/model_reducer.go:296`):

```go
func (m *Model) refreshDisplay() {
	m.filter = ""
	m.textInput.Reset()
	m.updateColumns()
	m.updateRows(true)
}
```

This unconditionally clears the active filter, and `updateRows(true)` →
`applyRows(autoSelect=true)` → `findContextIndex` only re-homes the cursor onto
the *scope* row (or the environment row), never the user's actual selection.

The user-visible bug: while a live (watched) category is on screen, each watch
tick reloads the data and wipes the user's active filter and jumps the cursor
back to the top / scope row.

## Root cause

The "clear filter" responsibility lives at **load time** (`refreshDisplay`),
which is shared by both navigation-driven loads and in-place refreshes.
Filtering and cursor position are *view state* that should survive a refresh of
the data you are already looking at; they should only reset when you navigate to
a different category.

Navigation already resets the rest of the view state — `sortColumn`, `sortAsc`,
`showFaulty`, `watching`, `watchTrigger` — in `updateCategoryCore`
(`internal/ui/tui/reducer_category.go:38-50`), but the filter reset was
piggybacked onto `refreshDisplay` instead.

## Design

Relocate the filter-clear from load time to navigation time, and make the
load-time refresh preserve filter + cursor.

### Scope decision

All data-load refreshes preserve filter + cursor. Only category **navigation**
clears them. This covers live-watch reload, manual same-category refresh, and
the post-edit refresh (`edit_tenant.go:313`) with one consistent rule, rather
than introducing a second refresh variant.

### Change 1 — navigation clears the filter

In `updateCategoryCore` (`reducer_category.go`), in the branch taken when the
category actually changes (`m.category != category`), reset the filter
alongside the existing view-state resets:

```go
m.filter = ""
m.textInput.Reset()
```

That branch already calls `m.applyRows(nil, nil, false)` to blank the table, so
no stale selection survives a category switch — which Change 3 relies on.

### Change 2 — `refreshDisplay` stops clearing the filter

Remove the `m.filter = ""` and `m.textInput.Reset()` lines. `refreshDisplay`
becomes a re-render that keeps the current filter applied:

```go
// refreshDisplay re-renders columns and rows for the current category,
// preserving the active filter and the user's selected row. The filter is
// only cleared on category navigation (updateCategoryCore).
func (m *Model) refreshDisplay() {
	m.updateColumns()
	m.updateRows(true)
}
```

### Change 3 — `applyRows` preserves the actual selected row

In `applyRows` (`model_reducer.go:95`), when `autoSelect` is true, prefer to
keep the cursor on the row the user had selected, identified by its Name cell
(`row[0]`):

1. Before installing the new rows, capture the current selection's identity via
   `m.selectedRawRow()[0]` (guard against an empty table / empty row).
2. After the new rows are set, choose the cursor index by priority:
   - index of the row whose `[0]` equals the captured name, else
   - `findContextIndex(rows)` (existing scope/environment behavior), else
   - top (`-1` → `GotoTop`).
3. Clamp to a valid index when the previously-selected row was filtered out or
   deleted by the reload.

On a fresh navigation load the table was just blanked via
`applyRows(nil, nil, false)`, so there is no prior selection to capture and the
cursor falls through to `findContextIndex` — preserving today's behavior. This
is what scopes cursor-preservation to in-place refreshes automatically, with no
extra flag.

## Consequences

- Live-watch reload keeps the active filter and the selected row.
- Manual same-category refresh and post-edit refresh now also preserve filter +
  cursor (intentional, per the scope decision).
- Switching categories still clears the filter and re-homes the cursor.

## Testing

- **Update** `TestApplyDataset_CurrentCategoryLoadRefreshes`
  (`model_reducer_test.go:81`): it currently asserts the filter is *cleared* on
  a current-category load. Invert it to assert the filter is **preserved**.
- **New**: a live-watch reload (`handleWatchTriggered` → reload → data handler)
  with a non-empty filter keeps the filter and keeps the cursor on the same row
  by identity.
- **New**: cursor clamps gracefully when the previously-selected row is absent
  from the reloaded rows (filtered out or deleted).
- **New**: category navigation (`updateCategoryCore`, category-changed branch)
  clears the filter and resets the input.

## Risk

Low–moderate. Blast radius is `refreshDisplay` / `applyRows` (all data loads)
and `updateCategoryCore` (all navigation). The behavior change is intentional
and documented by the inverted test. No new model fields — reuses
`selectedRawRow()`, `findContextIndex`, and `rawRows`.

## Out of scope

- Preserving scroll offset beyond what the existing `GotoTop` + `MoveDown`
  cursor-into-view logic provides.
- Any change to non-autoSelect refresh paths (e.g. layout-only updates).
