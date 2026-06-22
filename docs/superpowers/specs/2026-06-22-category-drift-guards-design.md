# Category Drift-Guards + Load-Path Consolidation — Design

**Date:** 2026-06-22
**Status:** Approved (pending implementation)
**Findings:** Code-quality review #3 (category behavior duplicated across dispatch planes) and #5 (overlapping old/new load-message paths).

## Problem

Per-category behavior is encoded across ~19 dispatch planes in ~14 files (domain
methods, the columns registry, a dozen TUI maps/switches, CLI `get`, MCP tools).
Several planes default-fall-through **silently** when a category is missing —
which is how the original GPUWorkload lazy-load bug shipped. Separately, the TUI
has a `dataMsg` type-switch whose per-category cases duplicate the typed
`*LoadedMsg` handlers, and that switch silently omits GPUWorkload.

The chosen scope (explicitly **not** a structural rewrite) is to:
1. Collapse the duplicated load-path so there is one per-category path (#5).
2. Add drift-guards across the partial planes so a future category that misses a
   plane **fails a test** instead of drifting silently (#3, root-cause mitigation).

## Non-goals

- No single "category descriptor" struct driving all planes (the larger #3
  rewrite). Deferred.
- No shared CLI/MCP/TUI loader resolver (the medium #3 option). Deferred.
- No behavior change to any user-facing command. The only production code change
  is removing dead branches in `handleDataMsg` (#5).

## Section 1 — Collapse the dual load-path (#5)

### Current state

- `dataMsg{Data any, Gen int}` has two **live** production roles:
  - `dataMsg{Data: *models.Dataset}` — the foundational Init load, produced via
    `datasetLoadedMsg` → `handleDataMsg` (`model_update.go`).
  - `dataMsg{}` (nil Data) — a refresh-display signal from `refreshDataCmd`
    (`reducer_category.go:100`).
- `handleDataMsg` (`model_reducer.go`) type-switches on `msg.Data` with
  per-category cases (`[]models.BaseModel`, `map[string][]models.ImportedModel`,
  `[]models.GPUPool` + `updateGPUPoolState()`, `map[string][]models.GPUNode`,
  `map[string][]models.DedicatedAICluster`, `models.TenancyOverrideGroup`).
  These cases are **dead in production**: every per-category load now flows
  through typed `*LoadedMsg` → `routeListLoadedMsg` → the typed handlers
  (`handleBaseModelsLoaded`, …). Only tests still drive the `dataMsg`
  per-category cases. The switch omits GPUWorkload — a latent asymmetry, not a
  live bug.

### Change

Reduce `handleDataMsg` to its live roles:

```go
func (m *Model) handleDataMsg(msg dataMsg) tea.Cmd {
    // gen-guard unchanged
    if msg.Gen != 0 && msg.Gen != m.gens.msg {
        m.endTask(true)
        return nil
    }
    if ds, ok := msg.Data.(*models.Dataset); ok {
        m.dataset = ds
        m.endTask(true)
        m.logger.Infow("data loaded", "category", m.category, "pendingTasks", m.pendingTasks)
    }
    m.refreshDisplay()
    return nil
}
```

- Removes the dead per-category cases. `updateGPUPoolState()` is unaffected — it
  is already invoked by the typed `handleGPUPoolsLoaded` path.
- `dataMsg` keeps both live roles; the per-category responsibility now lives
  solely in the typed family. The overlap (#5) and GPUWorkload omission are gone.

### Tests touched

- `TestProcessDataAndErrorMsg` (`model_test.go:261-269`) currently drives
  `handleDataMsg` with per-category `dataMsg` payloads. Repoint these at the
  typed handlers (`m.handleBaseModelsLoaded(...)`, `m.handleGPUPoolsLoaded(...)`,
  etc.) so real-path coverage is preserved rather than testing removed branches.
- `messages_test.go` (`dataMsg{Data: data}`) — adjust if it asserts a removed
  case; keep the `*models.Dataset` and empty-`dataMsg` assertions.
- Empty `dataMsg{}` usages (refresh signal) across other tests are unchanged.

### Guard for #5

A test asserting `handleDataMsg` only acts on `*models.Dataset`/refresh — i.e.
feeding it a per-category payload (e.g. `[]models.GPUPool`) does **not** mutate
the dataset (that path belongs to the typed handler). This locks in the
single-path invariant.

## Section 2 — Drift-guard catalog

New `*_drift_test.go` files, one per package, each iterating `domain.Categories`.
Mirrors the established pattern (`TestRowSources_CoversAllCategories`,
`internal/columns/registry_test.go`). **No production changes** beyond Section 1:
the command-constructor switches (`startK8sWatchCmd`, `reloadCategoryCmd`) build
their `tea.Cmd` lazily, so a test can call them per-category and assert
handled-vs-nil without triggering any watch or load.

Three guard styles:
- **Predicate invariant** — plane membership is derived from a domain truth; the
  guard self-maintains (no list to update).
- **Behavior-probe** — call the plane's function for each category and assert the
  expected handled/excluded outcome (for switches that can't be introspected).
- **Account-for-all** — for genuine per-category UI choices with no domain
  predicate: a table requires every category to be present or in an explicit
  exclusion set, so a new category fails until someone makes the decision.

### `internal/domain` — `category_drift_test.go`

- `NeedsKubeConfig`: account-for-all — `want := map[Category]bool{…}` with an
  entry for every `domain.Categories`; assert `len(want) == len(Categories)` and
  `want[c] == c.NeedsKubeConfig()` for each. New category → missing entry → fail.
- `ScopedCategories`/`Parents`: structural invariant — for every category `c` and
  every `child` in `c.ScopedCategories()`, assert `c` appears in
  `child.Parents()` (round-trip consistency, no list). Plus an account-for-all
  scope table asserting each category's `ScopedCategories()` matches an expected
  map covering all categories.
- `Aliases`: invariant — every category has ≥1 alias; aliases are unique across
  categories (no collisions); `ParseCategory(alias) == c` round-trips for every
  alias of every category.

### `internal/ui/tui` — `category_drift_test.go`

- `lazyLoadedCategories`: predicate — every `c.NeedsKubeConfig()` category is in
  the set. (Supersedes/absorbs the existing
  `TestLazyLoadedCategories_CoversKubeBacked`; remove the old one to avoid
  duplication.)
- `handlers` map (`reducer_category.go`): predicate — every kube-backed category
  has an entry.
- `startK8sWatchCmd`: behavior-probe — returns non-nil `tea.Cmd` for every
  kube-backed category and nil for non-kube categories.
- `reloadCategoryCmd`: behavior-probe — non-nil for the same kube-backed set.
- `statsColumns`: account-for-all — every category present or in an explicit
  `noStats` exclusion set; assert the union covers all categories.
- `itemKeyFrom`: behavior-probe — returns a non-nil `ItemKey` for every category,
  probing with a representative `table.Row`. If a synthetic row proves
  impractical for some categories, the implementer may instead assert that
  `itemKeyFrom`'s flat-vs-grouped classification accounts for every category via
  an account-for-all table; either mechanism must fail when a category is added
  without handling.
- `parentScope`: assert consistency with the domain scope graph — every category
  with `Parents()` is handled by `parentScope` (probe, or account-for-all over
  the scope-category set if a synthetic row is impractical).
- `deleteItem` ↔ `confirmDelete` consistency: the set of categories for which
  `deleteItem` returns non-nil equals the set `confirmDelete` assigns a tier to;
  assert they agree (guards the finding-#2 coupling from the confirmation work).

### `internal/ui/tui/keys` — `category_drift_test.go`

- `catContext`: account-for-all — every category present or in an explicit
  `noContextKeys` exclusion set; assert the union covers all categories.

### Already guarded (keep, do not duplicate)

- `columns/registry` (`registry_test.go`), `rowSources`
  (`TestRowSources_CoversAllCategories`).

## Architecture / isolation

- Each guard test is self-contained in its package's `*_drift_test.go`, reading
  unexported registries directly (same-package tests) — no new production
  exports.
- Explicit exclusion sets (`noStats`, `noContextKeys`, expected-value tables) live
  beside their guard test, documented with one line on why each category is
  excluded.

## Error handling

Guards are tests; failures surface at `make ci` / `go test`. No runtime paths
change. The #5 change removes branches only; the foundational-load and
refresh-signal paths are preserved exactly.

## Testing

- Section 1: the repointed `TestProcessDataAndErrorMsg`, the new single-path
  guard, and the existing TUI suite must stay green.
- Section 2: each new guard must **fail if a category is removed from its plane**
  — verify by temporarily dropping a known category from one plane and confirming
  the guard catches it (then revert). This proves the guard actually guards.
- Full `make ci` green (tests, lint, coverage threshold) on the branch.

## Out of scope / future

- The full category-descriptor refactor (#3 "Full") and the shared loader
  resolver (#3 "Moderate") remain available as later, larger efforts. These
  guards make that refactor safer by pinning current cross-plane invariants
  first.
