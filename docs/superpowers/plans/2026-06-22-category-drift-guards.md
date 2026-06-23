# Category Drift-Guards + Load-Path Consolidation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the duplicated per-category load-message path and add drift-guard tests across the partial category dispatch planes so a future category that misses a plane fails a test instead of drifting silently.

**Architecture:** One small production change (trim `handleDataMsg` to its live roles) plus one behavior-neutral extraction (move the category-handler maps to package scope so a test can inspect them); everything else is new `*_drift_test.go` files that iterate `domain.Categories` and assert each plane's coverage via predicate invariants, behavior-probes, or account-for-all tables.

**Tech Stack:** Go, testify, Bubble Tea.

## Global Constraints

- Code must pass `gofumpt -l` (zero diff), `golangci-lint run` (zero issues; cyclop max 13), and `go test ./...`.
- TDD: write the failing test first, watch it fail for the right reason, then implement.
- Tests reuse existing harnesses: `newTestModel(t)` (`model_test.go`), `watchableLoader` (`loader_cmd_watch_test.go`), and the `for _, cat := range domain.Categories` pattern (`export_csv_test.go:281`, `internal/columns/registry_test.go`).
- The kube-backed category set (from `domain.NeedsKubeConfig`) is exactly: `BaseModel`, `ImportedModel`, `GPUNode`, `DedicatedAICluster`, `GPUWorkload`.
- `domain.Categories` ranges `Tenant..Alias` (20 categories; excludes `CategoryUnknown`).
- **Spec deviation (approved in plan):** the spec said "no production changes beyond Section 1." Task 2 adds one more — extracting the local `handlers`/`tenancyOverrides` maps in `updateCategoryCore` to package scope. It is behavior-neutral (and avoids rebuilding the map every call) and is required to make the handlers plane testable.

---

### Task 1: Collapse the dual load-path (#5)

**Files:**
- Modify: `internal/ui/tui/model_reducer.go` (`handleDataMsg`)
- Test: `internal/ui/tui/model_reducer_test.go` (new), `internal/ui/tui/model_test.go` (`TestProcessDataAndErrorMsg`)

**Interfaces:**
- Consumes: `dataMsg{Data any, Gen int}`, `m.gens.msg`, `m.endTask`, `m.refreshDisplay`, typed handlers `handleBaseModelsLoaded`, `handleGPUPoolsLoaded`.
- Produces: a `handleDataMsg` that mutates the dataset only for `*models.Dataset` and otherwise just refreshes.

- [ ] **Step 1: Write the failing guard test**

Create `internal/ui/tui/model_reducer_test.go`:

```go
package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// handleDataMsg owns only the foundational dataset load and the refresh
// signal; per-category data flows through the typed *LoadedMsg handlers.
// Feeding it per-category data must NOT mutate the dataset.
func TestHandleDataMsg_IgnoresPerCategoryPayload(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{}

	m.handleDataMsg(dataMsg{Data: []models.GPUPool{{Name: "p1"}}})
	require.Empty(t, m.dataset.GPUPools, "per-category payload must not be applied by handleDataMsg")

	ds := &models.Dataset{}
	m.handleDataMsg(dataMsg{Data: ds})
	require.Same(t, ds, m.dataset, "foundational *models.Dataset payload must still be applied")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestHandleDataMsg_IgnoresPerCategoryPayload`
Expected: FAIL — the current `[]models.GPUPool` case sets `m.dataset.GPUPools`, so `require.Empty` fails.

- [ ] **Step 3: Trim handleDataMsg to its live roles**

In `internal/ui/tui/model_reducer.go`, replace the body of `handleDataMsg` with:

```go
func (m *Model) handleDataMsg(msg dataMsg) tea.Cmd {
	// Drop stale responses based on generation token (allow zero-value Gen).
	// Still endTask: the matching beginTask was already issued when the
	// load started, so the task must end to keep pendingTasks balanced.
	// Without this, a stale drop leaves the model permanently in
	// LoadingView — startup hang regression on `toolkit -c <lazy-cat>`.
	if msg.Gen != 0 && msg.Gen != m.gens.msg {
		m.endTask(true)
		return nil
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
	return nil
}
```

- [ ] **Step 4: Repoint TestProcessDataAndErrorMsg at the typed handlers**

In `internal/ui/tui/model_test.go`, replace the body of `TestProcessDataAndErrorMsg` (the per-category `handleDataMsg` lines) with calls to the real typed handlers so the live paths stay covered:

```go
func TestProcessDataAndErrorMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// handleDataMsg with the foundational dataset and the nil refresh signal.
	m.handleDataMsg(dataMsg{Data: m.dataset})
	m.handleDataMsg(dataMsg{})
	// Per-category data now flows through the typed handlers.
	m.handleBaseModelsLoaded([]models.BaseModel{{}}, m.gens.msg)
	m.handleGPUPoolsLoaded([]models.GPUPool{{}}, m.gens.msg)
	m.handleGPUNodesLoaded(map[string][]models.GPUNode{"pool": {}}, m.gens.msg)
	m.handleDedicatedAIClustersLoaded(map[string][]models.DedicatedAICluster{"tenant": {}}, m.gens.msg)
	// Update with errorMsg
	m.Update(errMsg{})
}
```

(If `messages_test.go` asserts a removed per-category `dataMsg` case, change it to assert the `*models.Dataset`/refresh behavior; leave empty-`dataMsg{}` usages untouched.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'TestHandleDataMsg_IgnoresPerCategoryPayload|TestProcessDataAndErrorMsg'`
Expected: PASS. Then `go test ./internal/ui/tui/` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/model_reducer.go internal/ui/tui/model_reducer_test.go internal/ui/tui/model_test.go
git commit -m "refactor(tui): collapse dual load-path into handleDataMsg live roles"
```

---

### Task 2: Extract category-handler maps to package scope

**Files:**
- Modify: `internal/ui/tui/reducer_category.go` (`updateCategoryCore`)
- Test: covered by the existing TUI suite + Task 3's guards.

**Interfaces:**
- Produces: package-level `type handlerFn func(*Model, bool, int) tea.Cmd`, `var categoryHandlers map[domain.Category]handlerFn`, `var tenancyOverrideCategories map[domain.Category]struct{}`. Task 3 reads `categoryHandlers`.

- [ ] **Step 1: Move the maps out of the function**

In `internal/ui/tui/reducer_category.go`, lift the `handlerFn` type and the two maps to package scope (above `updateCategoryCore`), renaming for clarity:

```go
// handlerFn loads/refreshes one category. Package-scoped so drift guards can
// assert coverage and so the map isn't rebuilt on every navigation.
type handlerFn func(*Model, bool, int) tea.Cmd

var categoryHandlers = map[domain.Category]handlerFn{
	domain.BaseModel:                       func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleBaseModelCategory(refresh, gen) },
	domain.ImportedModel:                   func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleImportedModelCategory(refresh, gen) },
	domain.GPUPool:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUPoolCategory(refresh, gen) },
	domain.GPUNode:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUNodeCategory(refresh, gen) },
	domain.GPUWorkload:                     func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUWorkloadCategory(refresh, gen) },
	domain.DedicatedAICluster:              func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleDedicatedAIClusterCategory(refresh, gen) },
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
```

Then in `updateCategoryCore`, delete the local `type handlerFn`, `handlers`, and `tenancyOverrides` declarations and update the two lookups to use the package vars:

```go
	if fn, ok := categoryHandlers[m.category]; ok {
		gen := m.bumpGen()
		cmd = fn(m, refresh, gen)
		if m.category.NeedsKubeConfig() {
			watchCmd = startK8sWatchCmd(m.loadCtx, m.loader, m.category, m.kubeConfig, m.environment, gen)
		}
	} else if _, ok := tenancyOverrideCategories[m.category]; ok {
		gen := m.bumpGen()
		cmd = m.handleTenancyOverridesGroup(gen)
	}
```

- [ ] **Step 2: Verify behavior is unchanged**

Run: `go test ./internal/ui/tui/` and `golangci-lint run ./internal/ui/tui/`
Expected: PASS / 0 issues (pure extraction; the existing category-navigation tests still pass).

- [ ] **Step 3: Commit**

```bash
git add internal/ui/tui/reducer_category.go
git commit -m "refactor(tui): lift category handler maps to package scope"
```

---

### Task 3: TUI load-lifecycle drift guards

**Files:**
- Create: `internal/ui/tui/category_drift_test.go`

**Interfaces:**
- Consumes: `lazyLoadedCategories`, `categoryHandlers` (Task 2), `(*Model).reloadCategoryCmd(cat, gen)`, `startK8sWatchCmd`, `watchableLoader`, `domain.Categories`, `domain.Category.NeedsKubeConfig`.

- [ ] **Step 1: Write the failing guards**

Create `internal/ui/tui/category_drift_test.go`:

```go
package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// kubeBackedCategories returns every category that loads from a live cluster.
func kubeBackedCategories() []domain.Category {
	var out []domain.Category
	for _, c := range domain.Categories {
		if c.NeedsKubeConfig() {
			out = append(out, c)
		}
	}
	return out
}

// Every kube-backed category must be lazy-loaded, have a load handler, and be
// reloadable/watchable — the cluster of planes the GPUWorkload bug missed.
func TestKubeBackedCategories_FullyWired(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range kubeBackedCategories() {
		_, lazy := lazyLoadedCategories[c]
		assert.Truef(t, lazy, "%s must be in lazyLoadedCategories", c)

		_, handled := categoryHandlers[c]
		assert.Truef(t, handled, "%s must have a categoryHandlers entry", c)

		assert.NotNilf(t, m.reloadCategoryCmd(c, 1), "%s must be reloadable", c)
	}
}

// reloadCategoryCmd returns a command only for kube-backed categories.
func TestReloadCategoryCmd_OnlyKubeBacked(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range domain.Categories {
		got := m.reloadCategoryCmd(c, 1)
		if c.NeedsKubeConfig() {
			assert.NotNilf(t, got, "%s is kube-backed and must reload", c)
		} else {
			assert.Nilf(t, got, "%s is not kube-backed and must not reload", c)
		}
	}
}

// startK8sWatchCmd must start a watch for every kube-backed category.
func TestStartK8sWatchCmd_CoversKubeBacked(t *testing.T) {
	t.Parallel()
	ld := &watchableLoader{}
	for _, c := range kubeBackedCategories() {
		cmd := startK8sWatchCmd(context.Background(), ld, c, "kc", models.Environment{}, 1)
		require.NotNilf(t, cmd, "%s watch cmd must be built", c)
		_, started := cmd().(k8sWatchStartedMsg)
		assert.Truef(t, started, "%s must produce k8sWatchStartedMsg, got unavailable", c)
	}
}
```

- [ ] **Step 2: Run to verify they pass (current code already satisfies them)**

Run: `go test ./internal/ui/tui/ -run 'TestKubeBackedCategories_FullyWired|TestReloadCategoryCmd_OnlyKubeBacked|TestStartK8sWatchCmd_CoversKubeBacked' -v`
Expected: PASS (these guard existing-correct wiring).

- [ ] **Step 3: Prove the guards actually guard**

Temporarily remove `domain.GPUWorkload: {},` from `lazyLoadedCategories` in `model.go`, re-run `TestKubeBackedCategories_FullyWired`. Expected: FAIL ("GPUWorkload must be in lazyLoadedCategories"). Then restore the line and confirm PASS.

- [ ] **Step 4: Remove the now-superseded older test**

Delete `TestLazyLoadedCategories_CoversKubeBacked` from `internal/ui/tui/model_test.go` — `TestKubeBackedCategories_FullyWired` subsumes it (lazy + handler + reload in one). Run `go test ./internal/ui/tui/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/category_drift_test.go internal/ui/tui/model_test.go
git commit -m "test(tui): drift-guard the kube-backed load-lifecycle planes"
```

---

### Task 4: TUI stats / key-shape / delete drift guards

**Files:**
- Modify: `internal/ui/tui/category_drift_test.go`

**Interfaces:**
- Consumes: `statsColumns`, `itemKeyFrom(cat, row)`, `parentScope(cat, row)`, `domain.Categories`, `domain.Category.Parents`.

- [ ] **Step 1: Write the failing guards**

Append to `internal/ui/tui/category_drift_test.go` (add `"github.com/charmbracelet/bubbles/table"` to imports):

```go
// noStatsCategories are intentionally without aggregate stat columns. A new
// category must be added here OR to statsColumns — never silently neither.
// Keep in sync with statsColumns (table_utils.go).
var noStatsCategories = map[domain.Category]struct{}{
	domain.Tenant:                          {},
	domain.LimitDefinition:                 {},
	domain.ConsolePropertyDefinition:       {},
	domain.PropertyDefinition:              {},
	domain.LimitTenancyOverride:            {},
	domain.ConsolePropertyTenancyOverride:  {},
	domain.PropertyTenancyOverride:         {},
	domain.LimitRegionalOverride:           {},
	domain.ConsolePropertyRegionalOverride: {},
	domain.PropertyRegionalOverride:        {},
	domain.BaseModel:                       {},
	domain.ImportedModel:                   {},
	domain.ModelArtifact:                   {},
	domain.Environment:                     {},
	domain.ServiceTenancy:                  {},
	domain.Alias:                           {},
}

// Every category either has stat columns or is explicitly listed as having
// none — a new category fails until someone decides.
func TestStatsColumns_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		_, hasStats := statsColumns[c]
		_, excluded := noStatsCategories[c]
		assert.Truef(t, hasStats != excluded,
			"%s must be in exactly one of statsColumns / noStatsCategories (hasStats=%v excluded=%v)",
			c, hasStats, excluded)
	}
}

// itemKeyFrom must produce a non-nil key for every category so selection works.
func TestItemKeyFrom_NonNilForEveryCategory(t *testing.T) {
	t.Parallel()
	row := table.Row{"a", "b", "c", "d"}
	for _, c := range domain.Categories {
		assert.NotNilf(t, itemKeyFrom(c, row), "itemKeyFrom returned nil for %s", c)
	}
}

// parentScope must resolve for every scoped (child) category.
func TestParentScope_ResolvesForScopedCategories(t *testing.T) {
	t.Parallel()
	row := table.Row{"a", "b", "c", "d"}
	for _, c := range domain.Categories {
		if len(c.Parents()) == 0 {
			continue
		}
		_, ok := parentScope(c, row)
		assert.Truef(t, ok, "parentScope must resolve a parent for scoped category %s", c)
	}
}
```

- [ ] **Step 2: Run and reconcile the account-for-all table**

Run: `go test ./internal/ui/tui/ -run 'TestStatsColumns_EveryCategoryAccountedFor|TestItemKeyFrom_NonNilForEveryCategory|TestParentScope_ResolvesForScopedCategories' -v`
Expected: PASS. If `TestStatsColumns...` fails, the failure names the category that is in neither map — fix `noStatsCategories` to match the actual `statsColumns` (table_utils.go:25), since the table above is derived, not read from source. If `TestParentScope...` fails for a category, the probe row shape is wrong for it — switch that assertion to an account-for-all over the scope-category set (the categories with `Parents()`) per the spec's fallback.

- [ ] **Step 3: Prove the stats guard guards**

Temporarily add `domain.Tenant` to `statsColumns` (so it's in both maps), re-run `TestStatsColumns_EveryCategoryAccountedFor`. Expected: FAIL (Tenant in both). Revert.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/tui/category_drift_test.go
git commit -m "test(tui): drift-guard stats columns, item keys, and parent scope"
```

---

### Task 5: domain drift guards

**Files:**
- Create: `internal/domain/category_drift_test.go`

**Interfaces:**
- Consumes: `domain.Categories`, `Category.NeedsKubeConfig`, `Category.ScopedCategories`, `Category.Parents`, `Category.Aliases`, `ParseCategory`.

- [ ] **Step 1: Write the failing guards**

Create `internal/domain/category_drift_test.go`:

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every category's kube-backed status must be a deliberate, listed choice.
func TestNeedsKubeConfig_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	want := map[Category]bool{
		Tenant:                          false,
		LimitDefinition:                 false,
		ConsolePropertyDefinition:       false,
		PropertyDefinition:              false,
		LimitTenancyOverride:            false,
		ConsolePropertyTenancyOverride:  false,
		PropertyTenancyOverride:         false,
		LimitRegionalOverride:           false,
		ConsolePropertyRegionalOverride: false,
		PropertyRegionalOverride:        false,
		BaseModel:                       true,
		ImportedModel:                   true,
		ModelArtifact:                   false,
		Environment:                     false,
		ServiceTenancy:                  false,
		GPUPool:                         false,
		GPUNode:                         true,
		GPUWorkload:                     true,
		DedicatedAICluster:              true,
		Alias:                           false,
	}
	require.Len(t, want, len(Categories), "every category must have an expected NeedsKubeConfig value")
	for _, c := range Categories {
		exp, ok := want[c]
		require.Truef(t, ok, "no expected NeedsKubeConfig value for %s", c)
		assert.Equalf(t, exp, c.NeedsKubeConfig(), "NeedsKubeConfig mismatch for %s", c)
	}
}

// Scope graph must be internally consistent: every child lists its parent back.
func TestScopeGraph_RoundTrips(t *testing.T) {
	t.Parallel()
	for _, parent := range Categories {
		for _, child := range parent.ScopedCategories() {
			assert.Containsf(t, child.Parents(), parent,
				"%s is scoped by %s but %s is not in %s.Parents()", child, parent, parent, child)
		}
	}
}

// Aliases must be unique across categories and round-trip through ParseCategory.
func TestAliases_UniqueAndRoundTrip(t *testing.T) {
	t.Parallel()
	seen := map[string]Category{}
	for _, c := range Categories {
		aliases := c.Aliases()
		assert.NotEmptyf(t, aliases, "%s must have at least one alias", c)
		for _, a := range aliases {
			if other, dup := seen[a]; dup {
				t.Errorf("alias %q is shared by %s and %s", a, other, c)
			}
			seen[a] = c
			got, err := ParseCategory(a)
			require.NoErrorf(t, err, "alias %q does not parse", a)
			assert.Equalf(t, c, got, "alias %q parses to %s, want %s", a, got, c)
		}
	}
}
```

- [ ] **Step 2: Run to verify they pass**

Run: `go test ./internal/domain/ -run 'TestNeedsKubeConfig_EveryCategoryAccountedFor|TestScopeGraph_RoundTrips|TestAliases_UniqueAndRoundTrip' -v`
Expected: PASS. (If `TestNeedsKubeConfig...` fails, reconcile `want` with `category.go:NeedsKubeConfig` — it is derived here, not read from source.)

- [ ] **Step 3: Prove the guard guards**

Temporarily delete the `Tenant:` line from `want`, re-run `TestNeedsKubeConfig_EveryCategoryAccountedFor`. Expected: FAIL (`require.Len` mismatch + "no expected value for Tenant"). Restore.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/category_drift_test.go
git commit -m "test(domain): drift-guard NeedsKubeConfig, scope graph, and aliases"
```

---

### Task 6: keys drift guard

**Files:**
- Create: `internal/ui/tui/keys/category_drift_test.go`

**Interfaces:**
- Consumes: `catContext` (package var in `keys`), `domain.Categories`.

- [ ] **Step 1: Write the failing guard**

Create `internal/ui/tui/keys/category_drift_test.go`:

```go
package keys

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
)

// noContextKeys are categories that intentionally have no per-category key
// bindings. A new category must be added here OR to catContext — never
// silently neither. Keep in sync with catContext (registry.go).
var noContextKeys = map[domain.Category]struct{}{
	domain.LimitDefinition: {},
	domain.ModelArtifact:   {},
	domain.Alias:           {},
}

func TestCatContext_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		_, hasKeys := catContext[c]
		_, excluded := noContextKeys[c]
		assert.Truef(t, hasKeys != excluded,
			"%s must be in exactly one of catContext / noContextKeys (hasKeys=%v excluded=%v)",
			c, hasKeys, excluded)
	}
}
```

- [ ] **Step 2: Run and reconcile**

Run: `go test ./internal/ui/tui/keys/ -run TestCatContext_EveryCategoryAccountedFor -v`
Expected: PASS. The `noContextKeys` set is derived (categories NOT keyed in `catContext`); if the test fails it names the mismatch — adjust `noContextKeys` to match the actual `catContext` keys in `registry.go:262`.

- [ ] **Step 3: Prove the guard guards**

Temporarily add `domain.Tenant` to `noContextKeys` (Tenant is also a `catContext` key), re-run. Expected: FAIL (Tenant in both). Revert.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/tui/keys/category_drift_test.go
git commit -m "test(keys): drift-guard per-category key bindings"
```

---

## Self-Review

**Spec coverage:**
- §1 collapse dual load-path → Task 1 (trim `handleDataMsg`, repoint tests, single-path guard). ✓
- §2 guard catalog:
  - `NeedsKubeConfig` account-for-all → Task 5. ✓
  - `ScopedCategories`/`Parents` round-trip + scope consistency → Task 5. ✓
  - `Aliases` uniqueness/round-trip → Task 5. ✓
  - `lazyLoadedCategories` predicate → Task 3. ✓
  - `handlers` predicate (needs package extraction) → Task 2 (extraction) + Task 3 (guard). ✓
  - `startK8sWatchCmd` / `reloadCategoryCmd` behavior-probe → Task 3. ✓
  - `statsColumns` account-for-all → Task 4. ✓
  - `itemKeyFrom` / `parentScope` probe → Task 4. ✓
  - `deleteItem`↔`confirmDelete` consistency → **see note below**.
  - `catContext` account-for-all → Task 6. ✓
  - `rowSources`/`columns` already guarded → untouched (kept). ✓

**Deletable-set consistency note:** the spec listed a `deleteItem`↔`confirmDelete` guard. On inspection both switch on exactly `{GPUNode, DedicatedAICluster}` and `confirmDelete`'s default is reached only for `DedicatedAICluster` (the `keys.Delete` binding lives in `catContext` only for those two — established in the confirmation-feature final review). A dedicated guard would have to probe `deleteItem` (needs a fully wired model+table) for marginal value over what `catContext`'s account-for-all (Task 6) and the existing confirmation tests already pin. **Dropped from the plan as low-value/high-setup; recorded here so the decision is explicit.** If desired later, add a `keys`-level assertion that exactly `{GPUNode, DedicatedAICluster}` bind `keys.Delete`.

**Placeholder scan:** none — every step has runnable code and exact commands. The two account-for-all tables (`noStatsCategories`, `noContextKeys`) are derived and explicitly self-reconcile on first run (Steps note this).

**Type consistency:** `categoryHandlers`/`handlerFn`/`tenancyOverrideCategories` (Task 2) are used identically in Task 3. `kubeBackedCategories()` is defined once (Task 3) and reused (Task 3). Probe signatures match the real sources: `reloadCategoryCmd(domain.Category, int) tea.Cmd`, `startK8sWatchCmd(context.Context, loader.Composite, domain.Category, string, models.Environment, int) tea.Cmd`, `itemKeyFrom(domain.Category, table.Row)`, `parentScope(domain.Category, table.Row) (domain.Scope, bool)`.
