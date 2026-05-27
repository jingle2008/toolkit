# Canonical Column Set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the parallel column definitions in `internal/cli/get.go` (table renderers) and `internal/ui/tui/{headers.go,row_builders.go}` into a single canonical registry under `internal/columns/`, exposing `toolkit get --columns` for power users.

**Architecture:** New top-level package `internal/columns/` holding one typed `Set[T]` or `GroupedSet[T]` per `domain.Category`. CLI (`internal/cli/get.go`) and TUI (`internal/ui/tui/table_utils.go`) both consume it through small adapters. The 12 per-category `*Table` functions and the `headerDefinitions` map go away.

**Tech Stack:** Go 1.22+ generics; cobra for CLI flag + completion; `internal/cli/output` for csv/tsv/table rendering; `github.com/charmbracelet/bubbles/table` for TUI rendering.

**Reference:** Design spec at `docs/superpowers/specs/2026-05-23-canonical-column-set-design.md`.

---

## Conventions used by this plan

- **Canonical column Title is Title Case** ("Name", "Display Name", "Capacity Type"). At CLI table render time the renderer uppercases each title (so today's `NAME, IDS, ...` headers are preserved). The TUI uses Titles as-is, preserving today's TUI headers.
- **Render bodies** in the column inventory tables are valid Go expressions. Wrap each into `func(t Type) string { return EXPR }` (or the grouped variant `func(k string, t Type) string { return EXPR }`) when transcribing.
- **`fmt.Sprint(b bool)` → `"true"|"false"`** — matches current `tenantToRow`. For `int`, prefer `strconv.Itoa`; CLI's `fmt.Sprintf("%d", ...)` is equivalent but more imports.
- **Each task ends with a verification command + commit**. Commits are small and frequent; do not squash unless the user asks.
- **Run `go build ./...` and `go test ./internal/...` after every task** even when not listed — the plan assumes a green tree at every commit boundary.

---

## Task 1: Bootstrap the `internal/columns` package

**Files:**
- Create: `internal/columns/columns.go`
- Create: `internal/columns/registry.go`
- Create: `internal/columns/registry_test.go`

- [ ] **Step 1: Write the failing consistency test**

Create `internal/columns/registry_test.go`:

```go
package columns

import (
	"math"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
)

// Every concrete Category must have a registered column set.
func TestRegistry_EveryCategoryRegistered(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown {
			continue
		}
		if !IsRegistered(cat) {
			t.Errorf("category %s has no registered column set", cat)
		}
	}
}

// Keys must be unique, non-empty; `help` is reserved.
func TestRegistry_KeysValid(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		keys := KeysFor(cat)
		seen := make(map[string]bool, len(keys))
		var hasDefault bool
		for _, k := range keys {
			if k == "" {
				t.Errorf("%s: empty key", cat)
			}
			if k == "help" {
				t.Errorf("%s: key %q is reserved", cat, k)
			}
			if seen[k] {
				t.Errorf("%s: duplicate key %q", cat, k)
			}
			seen[k] = true
		}
		// Defaults check goes via a separate accessor that returns
		// at least one true entry.
		for _, isDefault := range DefaultsFor(cat) {
			if isDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			t.Errorf("%s: no column has Default=true", cat)
		}
	}
}

// Ratios per set must sum to ~1.0 (±0.02).
func TestRegistry_RatiosSumToOne(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		sum := RatioSum(cat)
		if math.Abs(sum-1.0) > 0.02 {
			t.Errorf("%s: ratio sum %.3f outside ±0.02 of 1.0", cat, sum)
		}
	}
}
```

- [ ] **Step 2: Run the test, expect failure**

```bash
go test ./internal/columns/... -run TestRegistry -v
```

Expected: compile error (`columns` package doesn't exist).

- [ ] **Step 3: Implement `columns.go` with the type definitions**

Create `internal/columns/columns.go`:

```go
// Package columns is the single source of truth for the columns
// rendered by `toolkit get` (CLI table/csv/tsv) and the TUI table
// view. One Set or GroupedSet is defined per domain.Category; both
// surfaces consume them through adapters.
package columns

// Column is a column for a flat (non-grouped) category.
type Column[T any] struct {
	Title   string
	Key     string
	Default bool
	Ratio   float64
	Render  func(T) string
}

// GroupedColumn is a column for a grouped category (loader returns
// map[string][]T). Render receives both the group key and the item;
// any column can use either. A "group key column" is just a
// GroupedColumn whose Render ignores `item` and returns `key`.
type GroupedColumn[T any] struct {
	Title   string
	Key     string
	Default bool
	Ratio   float64
	Render  func(key string, item T) string
}

// Set is the canonical column list for a flat category.
type Set[T any] struct {
	Columns []Column[T]
}

// GroupedSet is the canonical column list for a grouped category.
type GroupedSet[T any] struct {
	Columns []GroupedColumn[T]
}

// DefaultColumns returns the columns of s where Default==true,
// in declared order.
func (s Set[T]) DefaultColumns() []Column[T] {
	out := make([]Column[T], 0, len(s.Columns))
	for _, c := range s.Columns {
		if c.Default {
			out = append(out, c)
		}
	}
	return out
}

// SelectColumns returns the columns of s whose Key is in keys,
// in the order given by keys. Returns an error listing all unknown
// keys (so the CLI can show a single complete message).
func (s Set[T]) SelectColumns(keys []string) ([]Column[T], error) {
	byKey := make(map[string]Column[T], len(s.Columns))
	for _, c := range s.Columns {
		byKey[c.Key] = c
	}
	out := make([]Column[T], 0, len(keys))
	var unknown []string
	for _, k := range keys {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		} else {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, &UnknownColumnError{Unknown: unknown, Valid: s.Keys()}
	}
	return out, nil
}

// Keys returns the keys declared on s in order.
func (s Set[T]) Keys() []string {
	out := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		out[i] = c.Key
	}
	return out
}

// DefaultColumns / SelectColumns / Keys mirrors for GroupedSet.
func (g GroupedSet[T]) DefaultColumns() []GroupedColumn[T] {
	out := make([]GroupedColumn[T], 0, len(g.Columns))
	for _, c := range g.Columns {
		if c.Default {
			out = append(out, c)
		}
	}
	return out
}

func (g GroupedSet[T]) SelectColumns(keys []string) ([]GroupedColumn[T], error) {
	byKey := make(map[string]GroupedColumn[T], len(g.Columns))
	for _, c := range g.Columns {
		byKey[c.Key] = c
	}
	out := make([]GroupedColumn[T], 0, len(keys))
	var unknown []string
	for _, k := range keys {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		} else {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, &UnknownColumnError{Unknown: unknown, Valid: g.Keys()}
	}
	return out, nil
}

func (g GroupedSet[T]) Keys() []string {
	out := make([]string, len(g.Columns))
	for i, c := range g.Columns {
		out[i] = c.Key
	}
	return out
}

// UnknownColumnError is returned by SelectColumns when one or more
// requested keys are not present in the set.
type UnknownColumnError struct {
	Unknown []string
	Valid   []string
}

func (e *UnknownColumnError) Error() string {
	return "unknown column key(s): " + joinComma(e.Unknown) +
		" (valid keys: " + joinComma(e.Valid) + ")"
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
```

- [ ] **Step 4: Implement `registry.go` with the dispatch + accessors**

Create `internal/columns/registry.go`:

```go
package columns

import (
	"fmt"
	"sort"

	"github.com/jingle2008/toolkit/internal/domain"
)

// IsRegistered reports whether cat has a canonical column set.
// Implementation lives alongside the per-category files; this
// switch is the single edit-site when a new category is added.
func IsRegistered(cat domain.Category) bool {
	switch cat { //nolint:exhaustive
	// Per-category files (added in later tasks) flip these on by
	// adding their case. Until then everything is unregistered.
	}
	return false
}

// KeysFor returns the declared keys for cat in order.
// Returns nil for unregistered categories.
func KeysFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// DefaultsFor returns the Default flag for each column of cat in
// declared order. The two slices KeysFor / DefaultsFor share the
// same indices; together they're enough to drive shell completion
// and the `--columns help` table.
func DefaultsFor(cat domain.Category) []bool {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// RatioSum returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSum(cat domain.Category) float64 {
	switch cat { //nolint:exhaustive
	}
	return 0
}

// RenderTable is the single entrypoint the CLI calls. It type-switches
// on cat, applies --columns selection, and produces headers+rows for
// the chosen encoding (table/csv/tsv). headers are uppercased to
// preserve today's CLI table headers (NAME, STATUS, ...); the TUI
// adapter (in internal/ui/tui) uses Titles as-is.
//
// `items` must be the concrete payload for cat. `selected` is the
// parsed --columns list (empty means "use Default columns").
//
//nolint:cyclop // a per-category switch is the contract here
func RenderTable(cat domain.Category, items any, selected []string) ([]string, [][]string, error) {
	switch cat { //nolint:exhaustive
	}
	return nil, nil, fmt.Errorf("category %s is not registered with the columns package", cat)
}

// HelpTable returns a (Key, Title, Default) row per column of cat,
// for the `--columns help` output. Empty if cat is unregistered.
func HelpTable(cat domain.Category) (headers []string, rows [][]string) {
	keys := KeysFor(cat)
	if keys == nil {
		return nil, nil
	}
	titles := TitlesFor(cat)
	defaults := DefaultsFor(cat)
	headers = []string{"KEY", "TITLE", "DEFAULT"}
	rows = make([][]string, len(keys))
	for i, k := range keys {
		def := "no"
		if defaults[i] {
			def = "yes"
		}
		rows[i] = []string{k, titles[i], def}
	}
	return headers, rows
}

// TitlesFor returns the Title for each column of cat in declared order.
func TitlesFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// sortedKeys returns the keys of a grouped map in sorted order so
// table output is deterministic.
func sortedKeys[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

Note: `strings.ToUpper` is applied per-column inside `renderFlat`/`renderGrouped` (added in Task 2 Step 4), which keeps the title-uppercasing decision local to the CLI render path.

The empty switches will be populated in subsequent tasks; for now `IsRegistered` always returns false, so the consistency tests skip everything.

- [ ] **Step 5: Run tests, expect green**

```bash
go test ./internal/columns/... -v
```

Expected: PASS (the loop bodies skip when `IsRegistered` returns false).

- [ ] **Step 6: Commit**

```bash
git add internal/columns/
git commit -m "feat(columns): bootstrap canonical column registry package

Adds Column[T], GroupedColumn[T], Set[T], GroupedSet[T] plus the
empty registry switches. No category is registered yet — that
happens in follow-up tasks.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: Port simple flat categories (Tenant, Alias, Environment, ServiceTenancy, LimitDefinition, LimitRegionalOverride)

**Files:**
- Create: `internal/columns/tenant.go`, `tenant_test.go`
- Create: `internal/columns/alias.go`, `alias_test.go`
- Create: `internal/columns/environment.go`, `environment_test.go`
- Create: `internal/columns/service_tenancy.go`, `service_tenancy_test.go`
- Create: `internal/columns/limit_definition.go`, `limit_definition_test.go`
- Create: `internal/columns/limit_regional_override.go`, `limit_regional_override_test.go`
- Modify: `internal/columns/registry.go` (add 6 cases to each switch)

### Worked example — Tenant

- [ ] **Step 1: Write failing test for Tenant**

Create `internal/columns/tenant_test.go`:

```go
package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTenantColumns(t *testing.T) {
	t.Parallel()
	tt := models.Tenant{
		Name:       "alpha",
		IDs:        []string{"ocid1.tenancy.oc1..a"},
		IsInternal: true,
		Note:       "n/a",
	}
	got := map[string]string{}
	for _, c := range TenantColumns.Columns {
		got[c.Key] = c.Render(tt)
	}
	want := map[string]string{
		"name":     "alpha",
		"ids":      "ocid1.tenancy.oc1..a",
		"internal": "true",
		"note":     "n/a",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
```

Run: `go test ./internal/columns/ -run TestTenantColumns -v` → FAIL (`TenantColumns` undefined).

- [ ] **Step 2: Create `tenant.go`**

```go
package columns

import (
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantColumns is the canonical column set for domain.Tenant.
// All columns Default==true (preserves today's CLI table NAME|IDS|INTERNAL|NOTE).
var TenantColumns = Set[models.Tenant]{Columns: []Column[models.Tenant]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.20,
		Render: func(t models.Tenant) string { return t.Name }},
	{Title: "OCID", Key: "ids", Default: true, Ratio: 0.60,
		Render: func(t models.Tenant) string { return strings.Join(t.IDs, ",") }},
	{Title: "Internal", Key: "internal", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return fmt.Sprint(t.IsInternal) }},
	{Title: "Note", Key: "note", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Note }},
}}
```

Note: Title "OCID" matches today's TUI; CLI's "IDS" header becomes "OCID" (uppercased to "OCID" — same length, no diff). Render uses `strings.Join` (CLI's content). TUI today renders `t.GetTenantID()` which returns the first or joined ID; switching to `strings.Join` is the intentional content unification.

- [ ] **Step 3: Wire Tenant into the registry switches**

Edit `internal/columns/registry.go`. Add a `domain.Tenant` case to each switch:

```go
// In IsRegistered:
case domain.Tenant:
    return true

// In KeysFor:
case domain.Tenant:
    return TenantColumns.Keys()

// In DefaultsFor:
case domain.Tenant:
    out := make([]bool, len(TenantColumns.Columns))
    for i, c := range TenantColumns.Columns {
        out[i] = c.Default
    }
    return out

// In TitlesFor:
case domain.Tenant:
    out := make([]string, len(TenantColumns.Columns))
    for i, c := range TenantColumns.Columns {
        out[i] = c.Title
    }
    return out

// In RatioSum:
case domain.Tenant:
    var sum float64
    for _, c := range TenantColumns.Columns {
        sum += c.Ratio
    }
    return sum

// In RenderTable: see Step 4 for the shared helper this calls.
case domain.Tenant:
    return renderFlat(TenantColumns, items, selected)
```

- [ ] **Step 4: Add the `renderFlat` / `renderGrouped` helpers to registry.go**

Add `"strings"` to the `registry.go` import block (used by both helpers for `strings.ToUpper`), then append the helpers:

```go
// renderFlat is the per-category branch body in RenderTable for
// flat categories. It picks defaults vs. selected, then runs each
// column's Render against each item.
func renderFlat[T any](s Set[T], items any, selected []string) ([]string, [][]string, error) {
	typed, ok := items.([]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderFlat: items has wrong type %T", items)
	}
	cols, err := pickFlat(s, selected)
	if err != nil {
		return nil, nil, err
	}
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = strings.ToUpper(c.Title)
	}
	rows := make([][]string, len(typed))
	for i, it := range typed {
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = c.Render(it)
		}
		rows[i] = row
	}
	return headers, rows, nil
}

func pickFlat[T any](s Set[T], selected []string) ([]Column[T], error) {
	if len(selected) == 0 {
		return s.DefaultColumns(), nil
	}
	return s.SelectColumns(selected)
}

// renderGrouped is the per-category branch body for grouped categories.
func renderGrouped[T any](g GroupedSet[T], items any, selected []string) ([]string, [][]string, error) {
	typed, ok := items.(map[string][]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderGrouped: items has wrong type %T", items)
	}
	cols, err := pickGrouped(g, selected)
	if err != nil {
		return nil, nil, err
	}
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = strings.ToUpper(c.Title)
	}
	total := 0
	for _, v := range typed {
		total += len(v)
	}
	rows := make([][]string, 0, total)
	for _, k := range sortedKeys(typed) {
		for _, it := range typed[k] {
			row := make([]string, len(cols))
			for j, c := range cols {
				row[j] = c.Render(k, it)
			}
			rows = append(rows, row)
		}
	}
	return headers, rows, nil
}

func pickGrouped[T any](g GroupedSet[T], selected []string) ([]GroupedColumn[T], error) {
	if len(selected) == 0 {
		return g.DefaultColumns(), nil
	}
	return g.SelectColumns(selected)
}
```

- [ ] **Step 5: Run Tenant tests, expect green**

```bash
go test ./internal/columns/ -run "TestTenantColumns|TestRegistry" -v
```

Expected: PASS.

### Remaining 5 categories — column inventories

Repeat steps 1–3 (test + file + registry case) for each. The render bodies below are valid Go expressions — wrap each in `func(x Type) string { return EXPR }`.

#### `alias.go` — `Set[domain.Category]`

Today CLI shape: 1 row per alias. Today TUI shape: 1 row per category with aliases joined. Canonical follows TUI (1 row per category).

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.40 | `c.String()` |
| Aliases | aliases | true | 0.60 | `strings.Join(c.GetAliases(), ", ")` |

`var AliasColumns = Set[domain.Category]{Columns: []Column[domain.Category]{...}}`

Note: this changes the CLI Alias shape from "1 row per alias" to "1 row per category" — intentional per spec Decision #4 generalization (canonical follows TUI).

Test fixture: `domain.Tenant` → Name="Tenant", Aliases="T, tenant".

#### `environment.go` — `Set[models.Environment]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.20 | `e.GetName()` |
| Realm | realm | true | 0.15 | `e.Realm` |
| Type | type | true | 0.15 | `e.Type` |
| Region | region | true | 0.50 | `e.Region` |

Test fixture: `models.Environment{Type: "preprod", Region: "us-ashburn-1", Realm: "oc1"}`. `GetName()` typically returns `"{Type}/{Region}"` or similar — assert via the same call.

#### `service_tenancy.go` — `Set[models.ServiceTenancy]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.15 | `s.Name` |
| Realm | realm | true | 0.10 | `s.Realm` |
| Environment | environment | true | 0.10 | `s.Environment` |
| Home Region | home-region | true | 0.15 | `s.HomeRegion` |
| Regions | regions | true | 0.50 | `strings.Join(s.Regions, ", ")` |

Note: TUI today mis-labels Environment as "Type"; canonical uses the accurate "Environment". The TUI header for this category changes — single TUI visible header change, acceptable as a bug fix.

#### `limit_definition.go` — `Set[models.LimitDefinition]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.32 | `d.Name` |
| Description | description | true | 0.48 | `d.Description` |
| Scope | scope | true | 0.08 | `d.Scope` |
| Min | min | true | 0.06 | `d.DefaultMin` |
| Max | max | true | 0.06 | `d.DefaultMax` |

#### `limit_regional_override.go` — `Set[models.LimitRegionalOverride]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.40 | `o.Name` |
| Regions | regions | true | 0.30 | `strings.Join(o.Regions, ", ")` |
| Min | min | false | 0.15 | `limitOverrideMin(o.Values)` |
| Max | max | false | 0.15 | `limitOverrideMax(o.Values)` |

`Min`/`Max` default to false (preserves CLI today: [Name, Regions]); TUI today shows all four.

Add helpers to `internal/columns/columns.go`:

```go
// limitOverrideMin returns Values[0].Min as a string, or "" when
// Values is empty (avoids the index-out-of-range that the current
// limitTenancyOverrideToRow assumes-away).
func limitOverrideMin(values []models.LimitRange) string {
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprint(values[0].Min)
}

func limitOverrideMax(values []models.LimitRange) string {
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprint(values[0].Max)
}
```

Place these in a small `helpers.go` if you'd rather not import `models` from `columns.go`:

```go
// internal/columns/helpers.go
package columns

import (
	"fmt"

	"github.com/jingle2008/toolkit/pkg/models"
)

func limitOverrideMin(values []models.LimitRange) string { /* as above */ }
func limitOverrideMax(values []models.LimitRange) string { /* as above */ }
```

- [ ] **Step 6: All Task-2 tests pass**

```bash
go test ./internal/columns/... -v
```

Expected: PASS for `TestTenantColumns`, `TestAliasColumns`, `TestEnvironmentColumns`, `TestServiceTenancyColumns`, `TestLimitDefinitionColumns`, `TestLimitRegionalOverrideColumns`, and all `TestRegistry*` consistency tests (now exercising 6 categories).

- [ ] **Step 7: Commit**

```bash
git add internal/columns/
git commit -m "feat(columns): port 6 flat categories (Tenant, Alias, Env, ST, LD, LRO)

Each carries Default==true for columns CLI shows today, plus any
TUI-only columns as Default==false. Alias canonical follows TUI's
1-row-per-category shape. ServiceTenancy fixes the TUI's
misleading 'Type' header to 'Environment'.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: Port complex flat categories (BaseModel, GPUPool)

**Files:**
- Create: `internal/columns/base_model.go`, `base_model_test.go`
- Create: `internal/columns/gpu_pool.go`, `gpu_pool_test.go`
- Modify: `internal/columns/registry.go` (add 2 cases per switch)

### `base_model.go` — `Set[models.BaseModel]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.20 | `m.Name` |
| Display Name | display-name | false | 0.22 | `m.DisplayName` |
| Internal | internal | true | 0.14 | `m.InternalName` |
| Vendor | vendor | true | 0.08 | `m.Vendor` |
| Type | type | true | 0.06 | `m.Type` |
| Version | version | true | 0.06 | `m.Version` |
| DAC Shape | dac-shape | false | 0.10 | `baseModelDacShape(m)` |
| Size | size | false | 0.05 | `m.ParameterSize` |
| Context | context | false | 0.05 | `strconv.Itoa(m.MaxTokens)` |
| Flags | flags | true | 0.07 | `m.GetFlags()` |
| Status | status | true | 0.04 | `m.Status` |

Helper (add to `internal/columns/base_model.go`):

```go
func baseModelDacShape(m models.BaseModel) string {
	shape := m.GetDefaultDacShape()
	if shape == nil {
		return ""
	}
	return fmt.Sprintf("%dx %s", shape.QuotaUnit, shape.Name)
}
```

Test fixture: a `models.BaseModel` with a non-nil DAC shape and `MaxTokens=4096`; assert `dac-shape == "1x foo.shape"`, `context == "4096"`, etc.

### `gpu_pool.go` — `Set[models.GPUPool]`

| Title | Key | Default | Ratio | Render |
|-------|-----|---------|-------|--------|
| Name | name | true | 0.22 | `p.Name` |
| Shape | shape | true | 0.20 | `p.Shape` |
| AD | ad | false | 0.06 | `p.AvailabilityDomain` |
| Size | size | true | 0.06 | `strconv.Itoa(p.Size)` |
| Actual Size | actual-size | true | 0.10 | `strconv.Itoa(p.ActualSize)` |
| GPUs | gpus | false | 0.06 | `strconv.Itoa(p.GetGPUs())` |
| OKE Managed | oke-managed | false | 0.10 | `strconv.FormatBool(p.IsOkeManaged)` |
| Capacity Type | capacity-type | true | 0.10 | `p.CapacityType` |
| Status | status | true | 0.10 | `p.Status` |

Test fixture: a populated `models.GPUPool` with `IsOkeManaged=true`, `Size=4`, `ActualSize=3`; assert each Render returns the expected value.

- [ ] **Step 1: Write failing test for BaseModel** (mirrors Task 2 Step 1).
- [ ] **Step 2: Create `base_model.go`** with the inventory above.
- [ ] **Step 3: Wire BaseModel into registry switches** (same shape as Task 2 Step 3).
- [ ] **Step 4: Repeat for GPUPool**.
- [ ] **Step 5: Run tests, expect green.**

```bash
go test ./internal/columns/ -v
```

- [ ] **Step 6: Commit.**

```bash
git commit -m "feat(columns): port BaseModel and GPUPool flat columns

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: Port the generic flat categories (Definition + RegionalOverride)

**Files:**
- Create: `internal/columns/definition.go`, `definition_test.go`
- Create: `internal/columns/regional_override.go`, `regional_override_test.go`
- Modify: `internal/columns/registry.go` (4 cases per switch — Console/Property × Definition/RegionalOverride)

### `definition.go` — generic over `models.Definition`

```go
package columns

import "github.com/jingle2008/toolkit/pkg/models"

// DefinitionColumns is parameterized by the concrete Definition
// type (ConsolePropertyDefinition or PropertyDefinition); both
// satisfy models.Definition.
func DefinitionColumns[T models.Definition]() Set[T] {
	return Set[T]{Columns: []Column[T]{
		{Title: "Name", Key: "name", Default: true, Ratio: 0.38,
			Render: func(d T) string { return d.GetName() }},
		{Title: "Description", Key: "description", Default: true, Ratio: 0.50,
			Render: func(d T) string { return d.GetDescription() }},
		{Title: "Value", Key: "value", Default: false, Ratio: 0.12,
			Render: func(d T) string { return d.GetValue() }},
	}}
}

// ConsolePropertyDefinitionColumns and PropertyDefinitionColumns
// are concrete instantiations so the registry switch can take
// their address without recomputing the closure every call.
var (
	ConsolePropertyDefinitionColumns = DefinitionColumns[models.ConsolePropertyDefinition]()
	PropertyDefinitionColumns        = DefinitionColumns[models.PropertyDefinition]()
)
```

CLI today shows [Name, Description] only; canonical default keeps that. TUI shows [Name, Description, Value] — canonical has Value as `Default==false`.

### `regional_override.go` — generic over `models.DefinitionOverride`

```go
package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

func RegionalOverrideColumns[T models.DefinitionOverride]() Set[T] {
	return Set[T]{Columns: []Column[T]{
		{Title: "Name", Key: "name", Default: true, Ratio: 0.40,
			Render: func(o T) string { return o.GetName() }},
		{Title: "Regions", Key: "regions", Default: true, Ratio: 0.40,
			Render: func(o T) string { return strings.Join(o.GetRegions(), ",") }},
		{Title: "Value", Key: "value", Default: false, Ratio: 0.20,
			Render: func(o T) string { return o.GetValue() }},
	}}
}

var (
	ConsolePropertyRegionalOverrideColumns = RegionalOverrideColumns[models.ConsolePropertyRegionalOverride]()
	PropertyRegionalOverrideColumns        = RegionalOverrideColumns[models.PropertyRegionalOverride]()
)
```

- [ ] **Step 1: Write failing tests** for each of `ConsolePropertyDefinitionColumns`, `PropertyDefinitionColumns`, `ConsolePropertyRegionalOverrideColumns`, `PropertyRegionalOverrideColumns`. One test per category, asserting the rendered cells against a small fixture (similar to Task 2 Step 1).

- [ ] **Step 2: Create `definition.go` and `regional_override.go`** as above.

- [ ] **Step 3: Wire all 4 categories into registry switches.** For example for `ConsolePropertyDefinition`:

```go
case domain.ConsolePropertyDefinition:
    return renderFlat(ConsolePropertyDefinitionColumns, items, selected)
```

…and the corresponding `KeysFor`, `DefaultsFor`, `TitlesFor`, `RatioSum`, `IsRegistered` cases.

- [ ] **Step 4: Run tests.**

```bash
go test ./internal/columns/ -v
```

- [ ] **Step 5: Commit.**

```bash
git commit -m "feat(columns): port Definition and RegionalOverride generic columns

Adds DefinitionColumns[T] and RegionalOverrideColumns[T] plus four
typed instantiations (Console/Property × Definition/Regional).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: Port grouped non-widened categories (GPUNode, DAC, ImportedModel, ModelArtifact)

**Files:**
- Create: `internal/columns/gpu_node.go`, `gpu_node_test.go`
- Create: `internal/columns/dac.go`, `dac_test.go`
- Create: `internal/columns/imported_model.go`, `imported_model_test.go`
- Create: `internal/columns/model_artifact.go`, `model_artifact_test.go`
- Modify: `internal/columns/registry.go` (4 grouped cases — switches now dispatch to `renderGrouped`)

All four use **name-first ordering** (per spec Decision #4) — the item name column comes first; the group key column comes second.

Exported variable names (used by registry switches and the TUI adapter): `GpuNodeColumns`, `DacColumns`, `ImportedModelColumns`, `ModelArtifactColumns`.

### `gpu_node.go` — `GroupedSet[models.GPUNode]`

| Title | Key | Default | Ratio | Render(k, n) |
|-------|-----|---------|-------|--------------|
| Name | name | true | 0.15 | `n.Name` |
| Pool | pool | true | 0.22 | `k` |
| Type | type | true | 0.15 | `n.InstanceType` |
| Total | total | false | 0.06 | `strconv.Itoa(n.Allocatable)` |
| Free | free | false | 0.06 | `strconv.Itoa(n.Allocatable - n.Allocated)` |
| Healthy | healthy | false | 0.06 | `strconv.FormatBool(n.IsHealthy())` |
| Ready | ready | false | 0.06 | `strconv.FormatBool(n.IsReady)` |
| Age | age | true | 0.06 | `n.Age` |
| Status | status | true | 0.18 | `n.GetStatus()` |

Defaults preserve today's CLI table content (POOL, NAME, STATUS, INSTANCE TYPE, AGE) — but reordered to name-first per the spec, and POOL is now the second column.

### `dac.go` — `GroupedSet[models.DedicatedAICluster]`

| Title | Key | Default | Ratio | Render(k, d) |
|-------|-----|---------|-------|--------------|
| Name | name | true | 0.35 | `d.Name` |
| Tenant | tenant | true | 0.16 | `k` |
| Internal | internal | false | 0.05 | `d.GetOwnerState()` |
| Usage | usage | false | 0.05 | `d.GetUsage()` |
| Type | type | true | 0.06 | `d.Type` |
| Model | model | true | 0.09 | `d.ModelName` |
| Shape/Profile | shape-profile | true | 0.12 | `dacUnitShapeOrProfile(d)` |
| Size | size | true | 0.04 | `strconv.Itoa(d.Size)` |
| Age | age | false | 0.04 | `d.Age` |
| Status | status | true | 0.04 | `d.Status` |

Helper:

```go
func dacUnitShapeOrProfile(d models.DedicatedAICluster) string {
	if d.UnitShape != "" {
		return d.UnitShape
	}
	return d.Profile
}
```

### `imported_model.go` — `GroupedSet[models.ImportedModel]`

The canonical set is the union of CLI today (NAME, NAMESPACE, VENDOR, VERSION, STATUS + TENANT key) and TUI today (Name, Tenant, Namespace, Display Name, Status). Vendor/Version live on the set as `Default==false` so power users can still surface them via `--columns`; today's TUI dropped them in commit e3a38d3 (Name+Display Name widened in their place).

| Title | Key | Default | Ratio | Render(k, m) |
|-------|-----|---------|-------|--------------|
| Name | name | true | 0.20 | `m.Name` |
| Tenant | tenant | true | 0.22 | `k` |
| Namespace | namespace | true | 0.15 | `m.Namespace` |
| Display Name | display-name | true | 0.27 | `m.DisplayName` |
| Vendor | vendor | false | 0.05 | `m.Vendor` |
| Version | version | false | 0.05 | `m.Version` |
| Status | status | true | 0.06 | `m.Status` |

Ratio sum = 1.00. CLI defaults drop Vendor/Version vs today (TUI parity); reachable via `--columns name,tenant,vendor,version,status`. This is one of the categories whose canonical CSV intentionally diffs against today's CLI output (Task 10 snapshot).

### `model_artifact.go` — `GroupedSet[models.ModelArtifact]`

| Title | Key | Default | Ratio | Render(k, a) |
|-------|-----|---------|-------|--------------|
| Name | name | true | 0.50 | `a.Name` |
| Model Internal Name | model-internal-name | true | 0.30 | `a.ModelName` |
| GPU Config | gpu-config | true | 0.10 | `a.GetGpuConfig()` |
| TensorRT | tensorrt | true | 0.10 | `a.TensorRTVersion` |

ModelArtifact's "key" is the parent BaseModel's name, which equals `a.ModelName` (the loader sets it). The canonical column doesn't render `k` explicitly — `a.ModelName` provides the same string in the "Model Internal Name" column.

Today's CLI table header was `MODEL` for the key column; the canonical Title is "Model Internal Name" (the TUI label). After uppercasing, CLI's first column becomes `NAME` and second becomes `MODEL INTERNAL NAME` (intentional diff: positional CSV consumers may break).

- [ ] **Step 1: Write failing tests** for each grouped category.

Example test for GPUNode:

```go
func TestGpuNodeColumns(t *testing.T) {
	t.Parallel()
	n := models.GPUNode{
		Name:         "node-1",
		NodePool:     "pool-A",
		InstanceType: "BM.GPU4.8",
		Allocatable:  8, Allocated: 3,
		IsReady: true,
		Age:     "1d",
	}
	got := map[string]string{}
	for _, c := range GpuNodeColumns.Columns {
		got[c.Key] = c.Render("pool-A", n)
	}
	if got["name"] != "node-1" || got["pool"] != "pool-A" || got["free"] != "5" {
		t.Errorf("unexpected renders: %+v", got)
	}
}
```

- [ ] **Step 2: Create the four `.go` files.**

Each follows this template:

```go
package columns

import (
	"strconv"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

var GpuNodeColumns = GroupedSet[models.GPUNode]{Columns: []GroupedColumn[models.GPUNode]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.15,
		Render: func(_ string, n models.GPUNode) string { return n.Name }},
	{Title: "Pool", Key: "pool", Default: true, Ratio: 0.22,
		Render: func(k string, _ models.GPUNode) string { return k }},
	// ...remaining columns from the inventory above
}}
```

(`strings` import only if used. Don't add unused imports — Go will refuse to compile.)

- [ ] **Step 3: Wire all 4 into registry.go.** Each `RenderTable` case calls `renderGrouped(GpuNodeColumns, items, selected)`.

- [ ] **Step 4: Run tests.**

```bash
go test ./internal/columns/ -v
```

- [ ] **Step 5: Commit.**

```bash
git commit -m "feat(columns): port grouped categories (GPUNode, DAC, IM, MA)

Canonical ordering is name-first, key-second (matches TUI). CLI
table output reorders for these 4 categories — intentional per
spec Decision #4.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: Port grouped widened categories (LimitTenancyOverride + generic Console/Property)

**Files:**
- Create: `internal/columns/limit_tenancy_override.go`, `limit_tenancy_override_test.go`
- Create: `internal/columns/tenancy_override.go`, `tenancy_override_test.go`
- Modify: `internal/columns/registry.go` (3 cases per switch — Limit + Console + Property)

Per spec Decision #9, all three categories' CLI defaults widen from today's [TENANT, NAME] to the TUI's full column list. Behavior change pinned by the snapshot test in Task 9.

### `limit_tenancy_override.go` — `GroupedSet[models.LimitTenancyOverride]`

| Title | Key | Default | Ratio | Render(k, v) |
|-------|-----|---------|-------|--------------|
| Name | name | true | 0.40 | `v.Name` |
| Tenant | tenant | true | 0.24 | `k` |
| Regions | regions | true | 0.20 | `strings.Join(v.Regions, ", ")` |
| Min | min | true | 0.08 | `limitOverrideMin(v.Values)` |
| Max | max | true | 0.08 | `limitOverrideMax(v.Values)` |

### `tenancy_override.go` — generic over `models.DefinitionOverride`

```go
func TenancyOverrideColumns[T models.DefinitionOverride]() GroupedSet[T] {
	return GroupedSet[T]{Columns: []GroupedColumn[T]{
		{Title: "Name", Key: "name", Default: true, Ratio: 0.40,
			Render: func(_ string, v T) string { return v.GetName() }},
		{Title: "Tenant", Key: "tenant", Default: true, Ratio: 0.25,
			Render: func(k string, _ T) string { return k }},
		{Title: "Regions", Key: "regions", Default: true, Ratio: 0.25,
			Render: func(_ string, v T) string { return strings.Join(v.GetRegions(), ", ") }},
		{Title: "Value", Key: "value", Default: true, Ratio: 0.10,
			Render: func(_ string, v T) string { return v.GetValue() }},
	}}
}

var (
	ConsolePropertyTenancyOverrideColumns = TenancyOverrideColumns[models.ConsolePropertyTenancyOverride]()
	PropertyTenancyOverrideColumns        = TenancyOverrideColumns[models.PropertyTenancyOverride]()
)
```

- [ ] **Step 1: Write failing tests for each of the 3 categories.**

- [ ] **Step 2: Create the two files.**

- [ ] **Step 3: Wire all 3 into registry.go.**

- [ ] **Step 4: Run all consistency tests + per-category tests.**

```bash
go test ./internal/columns/... -v
```

All 19 categories should now be registered. `TestRegistry_EveryCategoryRegistered` must pass without any skips for live categories.

- [ ] **Step 5: Commit.**

```bash
git commit -m "feat(columns): port tenancy override grouped columns

Defaults widen from [TENANT,NAME] to match the TUI (Name, Tenant,
Regions, Min/Max or Value). Intentional behavior change for
toolkit get -o table on these 3 categories — pinned by snapshot
test added in a later task.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: Wire `toolkit get` to use the registry; delete the old `*Table` functions

**Files:**
- Modify: `internal/cli/get.go`
- Delete: the 12 `*Table` functions in `internal/cli/get.go` (lines ~498-641 today: `tableFromSlice`, `tableFromGrouped`, `tenantTable`, `baseModelTable`, `importedModelTable`, `gpuPoolTable`, `gpuNodeTable`, `dacTable`, `tenancyOverrideTable`, `limitDefinitionTable`, `definitionTable`, `definitionOverrideTable`, `environmentTable`, `serviceTenancyTable`, `limitRegionalOverrideTable`, `modelArtifactTable`, plus helpers `sortedKeys`, `boolStr`, and `writeAliases`)
- Modify: `internal/cli/tables_test.go` — rewrite assertions to drive through the columns registry (the test file currently asserts against the deleted functions)

- [ ] **Step 1: Rewrite `emitCategory`** to thread `selected []string` into `writeSlice` / `writeMapFlat` / `writeMapWithKey` and call `columns.RenderTable` for the table/csv/tsv paths.

The new `writeSlice` becomes:

```go
func writeSlice[T any](w writer, items []T, limit int, opts output.Options, cat domain.Category, selected []string) error {
	items = collections.TruncateSlice(items, limit)
	switch opts.Format {
	case output.FormatJSON:
		return output.WriteJSON(w, items, opts)
	case output.FormatJSONL:
		return output.WriteJSONL(w, items, opts)
	case output.FormatYAML:
		return output.WriteYAML(w, items, opts)
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		headers, rows, err := columns.RenderTable(cat, items, selected)
		if err != nil {
			return err
		}
		return writeTableLike(w, headers, rows, opts)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

func writeTableLike(w writer, headers []string, rows [][]string, opts output.Options) error {
	switch opts.Format {
	case output.FormatTable:
		return output.WriteTable(w, headers, rows, opts)
	case output.FormatCSV:
		return output.WriteDelimited(w, headers, rows, opts, ',')
	case output.FormatTSV:
		return output.WriteDelimited(w, headers, rows, opts, '\t')
	}
	return fmt.Errorf("writeTableLike: unsupported %q", opts.Format)
}
```

`writeMapFlat` and `writeMapWithKey` collapse into `writeMap` (since the canonical set handles both the "key on item" and "key injected" cases via `GroupedColumn.Render(k, item)`):

```go
func writeMap[T any](w writer, grouped map[string][]T, limit int, opts output.Options, cat domain.Category, selected []string) error {
	switch opts.Format {
	case output.FormatJSON, output.FormatJSONL, output.FormatYAML:
		return writeEncoded(w, opts, collections.TruncateSlice(output.Flatten(grouped), limit))
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		headers, rows, err := columns.RenderTable(cat, grouped, selected)
		if err != nil {
			return err
		}
		rows = collections.TruncateSlice(rows, limit)
		return writeTableLike(w, headers, rows, opts)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}
```

`emitCategory`, `emitTenancyGroup`, `emitFromDataset`, `writeAliases` all update to pass `cat` + `selected` instead of a `toTable` callback. The `*Table` callbacks and `tableFrom{Slice,Grouped}` helpers are deleted. The `writeAliases` helper becomes a thin wrapper that builds `[]domain.Category` then calls `writeSlice`.

- [ ] **Step 2: Stub `--columns` flag wiring** — for this task, just thread `selected []string` from a not-yet-flag-wired value (default nil) through `runGet`. The flag itself is added in Task 9.

```go
// runGet — add `selected: nil` param when calling emitCategory.
```

- [ ] **Step 3: Update `internal/cli/tables_test.go`**

Today this test asserts headers/rows against the per-category `*Table` functions. After this task those functions don't exist. Rewrite each subtest to call `columns.RenderTable(cat, items, nil)` and assert the same headers (uppercased Titles) and row content.

For categories whose Default columns now produce different content (the 3 widened tenancy overrides) or different ordering (4 grouped), update the assertions accordingly. The test file becomes a per-category integration check that the registry returns what the spec mandates.

- [ ] **Step 4: Run all tests.**

```bash
go test ./... -count=1
```

Expected: PASS. If any CLI test fails because it called a deleted `*Table` function directly, replace with `columns.RenderTable`.

- [ ] **Step 5: Run `toolkit get` smoke checks against a real dataset** (skip if no dataset is locally available; manual step).

```bash
go build -o /tmp/toolkit ./cmd/toolkit
/tmp/toolkit get tenant -o csv | head
/tmp/toolkit get gpunode -o csv | head
/tmp/toolkit get lto -o csv | head  # widened
```

- [ ] **Step 6: Commit.**

```bash
git commit -m "refactor(cli): use canonical column registry; drop *Table fns

internal/cli/get.go now consults internal/columns for all
table/csv/tsv output. Per-category *Table functions and helpers
deleted. writeMapFlat/writeMapWithKey collapse into writeMap
because GroupedColumn.Render handles both flatten styles.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: Wire the TUI to use the registry; delete `headers.go` and `row_builders.go`

**Files:**
- Create: `internal/ui/tui/columns_adapter.go` (or similar) — the small `tuiColumns*` / `tuiRows*` adapter functions
- Modify: `internal/ui/tui/table_utils.go` — `getHeaders` and `categoryHandlers` switch to adapter calls
- Delete: `internal/ui/tui/headers.go`
- Delete: `internal/ui/tui/headers_test.go` (or rewrite against the adapter)
- Delete: `internal/ui/tui/row_builders.go`
- Delete: `internal/ui/tui/row_builders_test.go` (or rewrite)
- Modify: `internal/ui/tui/export_csv.go` — its one reference to `dedicatedAIClusterToRowInternal` (line 65) needs to migrate to the canonical renderer

- [ ] **Step 1: Add adapter functions.**

Create `internal/ui/tui/columns_adapter.go`:

```go
package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// tuiColumnsFlat converts a canonical Set into the TUI's
// []table.Column shape, with widths derived from each column's Ratio
// and the provided totalWidth.
func tuiColumnsFlat[T any](s columns.Set[T], totalWidth int) []table.Column {
	out := make([]table.Column, len(s.Columns))
	for i, c := range s.Columns {
		out[i] = table.Column{Title: c.Title, Width: int(c.Ratio * float64(totalWidth))}
	}
	return out
}

func tuiColumnsGrouped[T any](g columns.GroupedSet[T], totalWidth int) []table.Column {
	out := make([]table.Column, len(g.Columns))
	for i, c := range g.Columns {
		out[i] = table.Column{Title: c.Title, Width: int(c.Ratio * float64(totalWidth))}
	}
	return out
}

// tuiRowsFlat renders a slice through a flat Set. The filter / faulty
// gates stay in filterRows (it already knows about models.Faulty).
func tuiRowsFlat[T models.NamedFilterable](s columns.Set[T], items []T, filter string, faultyOnly bool) []table.Row {
	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
	}
	matches := collections.FilterSlice(items, nil, filter, pred)
	rows := make([]table.Row, len(matches))
	for i, m := range matches {
		row := make(table.Row, len(s.Columns))
		for j, c := range s.Columns {
			row[j] = c.Render(m)
		}
		rows[i] = row
	}
	return rows
}

// tuiRowsGrouped renders a grouped map. The filter + scope logic
// reuses filterRowsScoped's existing key/name routing.
func tuiRowsGrouped[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
	faultyOnly bool,
) []table.Row {
	var (
		key  *string
		name *string
	)
	if ctx != nil {
		if ctx.Category == scopeCategory {
			key = &ctx.Name
		} else {
			name = &ctx.Name
		}
	}
	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
	}
	matches := collections.FilterMap(m, key, name, filter, pred)
	rows := make([]table.Row, 0)
	for k, items := range matches {
		for _, it := range items {
			row := make(table.Row, len(g.Columns))
			for j, c := range g.Columns {
				row[j] = c.Render(k, it)
			}
			rows = append(rows, row)
		}
	}
	return rows
}
```

- [ ] **Step 2: Rewrite `table_utils.go` `getHeaders` to derive from the canonical sets, preserving the `[]header` return type.**

`getHeaders` today returns `[]header{text, ratio}` and is consumed in four places:
- `model_reducer.go:112` — stores it as `m.headers`
- `export_csv.go:48` — iterates `m.headers[i].text` for CSV column titles
- `table_utils.go:113, 143, 169` — column-by-title lookups for sort, numeric stats, and DAC Status stats

All four consume `text`/`ratio` only. The simplest migration keeps the signature; only the body changes:

```go
// In internal/ui/tui/table_utils.go — replace the body of getHeaders.
func getHeaders(cat domain.Category) []header {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return headersFromSet(columns.TenantColumns.Columns)
	case domain.LimitDefinition:
		return headersFromSet(columns.LimitDefinitionColumns.Columns)
	case domain.ConsolePropertyDefinition:
		return headersFromSet(columns.ConsolePropertyDefinitionColumns.Columns)
	case domain.PropertyDefinition:
		return headersFromSet(columns.PropertyDefinitionColumns.Columns)
	case domain.LimitRegionalOverride:
		return headersFromSet(columns.LimitRegionalOverrideColumns.Columns)
	case domain.ConsolePropertyRegionalOverride:
		return headersFromSet(columns.ConsolePropertyRegionalOverrideColumns.Columns)
	case domain.PropertyRegionalOverride:
		return headersFromSet(columns.PropertyRegionalOverrideColumns.Columns)
	case domain.BaseModel:
		return headersFromSet(columns.BaseModelColumns.Columns)
	case domain.Environment:
		return headersFromSet(columns.EnvironmentColumns.Columns)
	case domain.ServiceTenancy:
		return headersFromSet(columns.ServiceTenancyColumns.Columns)
	case domain.GPUPool:
		return headersFromSet(columns.GpuPoolColumns.Columns)
	case domain.Alias:
		return headersFromSet(columns.AliasColumns.Columns)
	case domain.GPUNode:
		return headersFromGroupedSet(columns.GpuNodeColumns.Columns)
	case domain.DedicatedAICluster:
		return headersFromGroupedSet(columns.DacColumns.Columns)
	case domain.ImportedModel:
		return headersFromGroupedSet(columns.ImportedModelColumns.Columns)
	case domain.ModelArtifact:
		return headersFromGroupedSet(columns.ModelArtifactColumns.Columns)
	case domain.LimitTenancyOverride:
		return headersFromGroupedSet(columns.LimitTenancyOverrideColumns.Columns)
	case domain.ConsolePropertyTenancyOverride:
		return headersFromGroupedSet(columns.ConsolePropertyTenancyOverrideColumns.Columns)
	case domain.PropertyTenancyOverride:
		return headersFromGroupedSet(columns.PropertyTenancyOverrideColumns.Columns)
	}
	return nil
}

func headersFromSet[T any](cols []columns.Column[T]) []header {
	out := make([]header, len(cols))
	for i, c := range cols {
		out[i] = header{text: c.Title, ratio: c.Ratio}
	}
	return out
}

func headersFromGroupedSet[T any](cols []columns.GroupedColumn[T]) []header {
	out := make([]header, len(cols))
	for i, c := range cols {
		out[i] = header{text: c.Title, ratio: c.Ratio}
	}
	return out
}
```

`categoryHandlers` becomes a per-category switch on the same set of canonical column references; replace each handler closure with a call to `tuiRowsFlat` (flat) or `tuiRowsGrouped` (grouped). The `aliasToRow / tenantToRow / ...` callbacks are dropped — they live only inside `row_builders.go`, which gets deleted in Step 4.

`computeNumericStats` and `appendDedicatedAIClusterStats` look up columns by Title text. The canonical Titles preserve the existing strings (Total, Free, Size, GPUs, Status) so neither helper needs changes.

- [ ] **Step 3: Update `internal/ui/tui/export_csv.go` — preserve the DAC-ID substitution path.**

The DAC CSV export today calls `dedicatedAIClusterToRowInternal(val, val.GetTenantID(realm), &id)` so the first cell becomes `val.GetID(realm, region)` instead of `val.Name`. This is a CSV-only override; the canonical `DacColumns` renders `d.Name` by design.

Keep a small private helper inside `export_csv.go` that mirrors the canonical DAC columns but substitutes the first cell. Replace the inline closure body in `writeCSV`:

```go
// In internal/ui/tui/export_csv.go, inside writeCSV when category == DAC.
rows = filterRowsScoped(
    m.dataset.DedicatedAIClusterMap, domain.Tenant,
    m.context, m.curFilter, m.showFaulty,
    func(val models.DedicatedAICluster, tenant string) table.Row {
        row := make(table.Row, len(columns.DacColumns.Columns))
        for i, c := range columns.DacColumns.Columns {
            row[i] = c.Render(tenant, val)
        }
        // Substitute the Name column with the realm/region-qualified ID.
        // DacColumns places Name at index 0 by design.
        row[0] = val.GetID(realm, region)
        // Substitute the Tenant column with the realm-resolved tenant ID.
        // DacColumns places Tenant at index 1.
        row[1] = val.GetTenantID(realm)
        return row
    })
```

The `row[0]`/`row[1]` indices are documented invariants of `DacColumns` ordering. Add a comment to `internal/columns/dac.go` warning future editors not to reorder Name/Tenant without updating `export_csv.go`:

```go
// DacColumns ordering invariant: row[0]=Name, row[1]=Tenant.
// internal/ui/tui/export_csv.go depends on this ordering when
// substituting the realm/region-qualified ID for the Name column
// in CSV export. If you reorder, fix the substitution there too.
```

- [ ] **Step 3a: Add adapter unit tests.**

Create `internal/ui/tui/columns_adapter_test.go`:

```go
package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTuiColumnsFlat_BaseModelWidth(t *testing.T) {
	t.Parallel()
	got := tuiColumnsFlat(columns.BaseModelColumns, 100)
	if len(got) != len(columns.BaseModelColumns.Columns) {
		t.Fatalf("len: got %d, want %d", len(got), len(columns.BaseModelColumns.Columns))
	}
	if got[0].Title != "Name" {
		t.Errorf("first Title: got %q, want Name", got[0].Title)
	}
	// Ratio 0.20 of 100 = 20.
	if got[0].Width != 20 {
		t.Errorf("first Width: got %d, want 20", got[0].Width)
	}
}

func TestTuiRowsGrouped_GpuNode(t *testing.T) {
	t.Parallel()
	m := map[string][]models.GPUNode{
		"pool-A": {{Name: "n1", InstanceType: "BM.GPU4.8", Allocatable: 8, Allocated: 1, IsReady: true, Age: "1d"}},
	}
	rows := tuiRowsGrouped(columns.GpuNodeColumns, m, 0, nil, "", false)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d, want 1", len(rows))
	}
	if rows[0][0] != "n1" || rows[0][1] != "pool-A" {
		t.Errorf("name/pool: got %v", rows[0])
	}
}
```

Run: `go test ./internal/ui/tui/ -run "TestTuiColumns|TestTuiRows" -v` → PASS.

- [ ] **Step 4: Delete `headers.go`, `headers_test.go`, `row_builders.go`, `row_builders_test.go`.**

```bash
git rm internal/ui/tui/headers.go internal/ui/tui/headers_test.go \
       internal/ui/tui/row_builders.go internal/ui/tui/row_builders_test.go
```

If any of those tests covered behavior worth keeping (e.g., specific cell formatting), rewrite the assertions in `internal/columns/<category>_test.go` so coverage is preserved by the new tests.

- [ ] **Step 5: Run all tests + start the TUI manually.**

```bash
go test ./... -count=1
go build -o /tmp/toolkit ./cmd/toolkit
/tmp/toolkit  # interactive — verify category switches render
```

Expected: all tests pass; TUI columns/widths look the same as before for the 19 categories.

- [ ] **Step 6: Commit.**

```bash
git commit -m "refactor(tui): consume canonical column registry

Deletes internal/ui/tui/headers.go and row_builders.go; getHeaders
and categoryHandlers now adapt directly from internal/columns
Set/GroupedSet. TUI visual output preserved.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 9: Add the `--columns` flag (parsing, validation, completion, help, mutex with structured outputs)

**Files:**
- Modify: `internal/cli/get.go` — add flag definition, parsing, mutual-exclusion check
- Create: `internal/cli/get_columns_test.go` — flag-level tests

- [ ] **Step 1: Write failing test for the `--columns` flag.**

Create `internal/cli/get_columns_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestGet_ColumnsFlag_Defaults(t *testing.T) {
	// run `toolkit get alias -o csv` with no --columns, expect 2 columns.
	out, err := runGetCmd(t, "alias", "-o", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	header := strings.SplitN(out, "\n", 2)[0]
	got := strings.Split(header, ",")
	if len(got) != 2 || got[0] != "NAME" || got[1] != "ALIASES" {
		t.Errorf("default header = %q, want NAME,ALIASES", header)
	}
}

func TestGet_ColumnsFlag_Explicit(t *testing.T) {
	out, err := runGetCmd(t, "alias", "-o", "csv", "--columns", "aliases,name")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	header := strings.SplitN(out, "\n", 2)[0]
	if got := strings.Split(header, ","); got[0] != "ALIASES" || got[1] != "NAME" {
		t.Errorf("header = %q, want ALIASES,NAME", header)
	}
}

func TestGet_ColumnsFlag_Unknown(t *testing.T) {
	_, err := runGetCmd(t, "alias", "-o", "csv", "--columns", "name,bogus")
	if err == nil {
		t.Fatal("expected error for unknown column, got nil")
	}
	if !strings.Contains(err.Error(), "unknown column key(s): bogus") {
		t.Errorf("error %q does not mention unknown key", err.Error())
	}
	if !strings.Contains(err.Error(), "valid keys:") {
		t.Errorf("error %q does not list valid keys", err.Error())
	}
}

func TestGet_ColumnsFlag_EmptyToken(t *testing.T) {
	_, err := runGetCmd(t, "alias", "-o", "csv", "--columns", "name,,aliases")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestGet_ColumnsFlag_Help(t *testing.T) {
	out, err := runGetCmd(t, "alias", "--columns", "help")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "KEY") || !strings.Contains(out, "TITLE") || !strings.Contains(out, "DEFAULT") {
		t.Errorf("help output missing expected headers: %s", out)
	}
}

func TestGet_ColumnsFlag_MutexWithJSON(t *testing.T) {
	_, err := runGetCmd(t, "alias", "-o", "json", "--columns", "name")
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--columns has no effect with -o json") {
		t.Errorf("error message: %q", err.Error())
	}
}

// runGetCmd invokes the root command with the given args; returns
// stdout on success, error on non-zero exit. Mirrors the pattern in
// internal/cli/get_test.go (TestGetCmd_UnknownCategory, etc).
func runGetCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	cmd := NewRootCmd("vtest")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"get"}, args...))
	err := cmd.Execute()
	return out.String(), err
}
```

Note: The `alias` category does not need a loader, so `TestGet_ColumnsFlag_*` tests can run against it without setting up `repo_path` or kubeconfig. The existing `TestGetCmd_UnknownCategory` test confirms `NewRootCmd("vtest")` is the canonical test setup.

Run: `go test ./internal/cli/ -run TestGet_ColumnsFlag -v` → FAIL (flag not defined).

- [ ] **Step 2: Add the flag to `addGetCommand`.**

In `internal/cli/get.go`:

```go
func addGetCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		format     string
		noHeaders  bool
		pretty     bool
		limit      int
		columnsArg string
	)
	// ...existing setup...
	getCmd.Flags().StringVar(&columnsArg, "columns", "",
		"comma-separated column keys (default: category's Default columns). Use --columns help to list valid keys.")
	_ = getCmd.RegisterFlagCompletionFunc("columns", func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) < 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return columns.KeysFor(cat), cobra.ShellCompDirectiveNoFileComp
	})
	getCmd.RunE = runGet(cfgFile, &format, &noHeaders, &pretty, &limit, &columnsArg)
	// ...rest unchanged...
}
```

- [ ] **Step 3: Parse `--columns` in `runGet` and thread through.**

```go
func runGet(cfgFile *string, format *string, noHeaders, pretty *bool, limit *int, columnsArg *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return fmt.Errorf("unknown category %q (run `toolkit get -h` for examples)", args[0])
		}

		// --columns help short-circuit.
		if *columnsArg == "help" {
			headers, rows := columns.HelpTable(cat)
			return output.WriteTable(cmd.OutOrStdout(), headers, rows, output.Options{})
		}

		fmtChoice, err := output.ParseFormat(*format)
		if err != nil {
			return err
		}

		selected, err := parseColumns(*columnsArg)
		if err != nil {
			return err
		}
		if len(selected) > 0 && !isTableLike(fmtChoice) {
			return fmt.Errorf("--columns has no effect with -o %s; remove the flag or switch to -o table/csv/tsv", fmtChoice)
		}

		// ...rest of runGet unchanged, except pass `selected` down...
	}
}

// parseColumns splits "name, status" → ["name","status"], trimming
// whitespace. Empty tokens (e.g. "name,,status") are an error.
func parseColumns(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			return nil, fmt.Errorf("--columns: empty token in %q", s)
		}
		out = append(out, t)
	}
	return out, nil
}

func isTableLike(f output.Format) bool {
	switch f {
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		return true
	}
	return false
}
```

- [ ] **Step 4: Thread `selected` into emitCategory and downstream.**

Update the signatures from Task 7 to actually receive the parsed `selected []string` from `runGet`.

- [ ] **Step 5: Run flag tests + full suite.**

```bash
go test ./internal/cli/... -v
go test ./... -count=1
```

- [ ] **Step 6: Update the help text in `getCmd.Long`.**

Add an example to the existing examples block:

```
  toolkit get gpunode --columns name,status,total,free
  toolkit get basemodel --columns help
```

- [ ] **Step 7: Commit.**

```bash
git commit -m "feat(cli): add --columns flag to toolkit get

Power-user column projection on top of the canonical registry.
Defaults preserved; --columns help lists valid keys; structured
outputs (json/jsonl/yaml) reject the flag with a clear message.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 10: Behavior-preservation snapshot test

**Files:**
- Create: `internal/cli/snapshot_test.go`
- Create: `internal/cli/testdata/snapshots/<category>.csv` (one file per category, committed)
- Optional: `internal/cli/testdata/dataset/` if a fixture dataset doesn't already exist

- [ ] **Step 1: Build a fixed test dataset.**

Check existing fixtures first:

```bash
grep -rn "fakeLoader\|FakeLoader\|inMemoryLoader\|NewTestLoader" internal/ pkg/ | head
ls internal/infra/loader/ 2>/dev/null
```

If a fake loader exists, reuse it. If not, the snapshot test can drive `columns.RenderTable` directly with hand-built typed slices/maps, bypassing the loader entirely:

```go
func renderSnapshot(t *testing.T, cat domain.Category) string {
	items := fixtureFor(cat)  // returns []T or map[string][]T
	headers, rows, err := columns.RenderTable(cat, items, nil)
	if err != nil {
		t.Fatalf("RenderTable: %v", err)
	}
	var buf bytes.Buffer
	if err := output.WriteDelimited(&buf, headers, rows, output.Options{}, ','); err != nil {
		t.Fatalf("WriteDelimited: %v", err)
	}
	return buf.String()
}
```

This sidesteps the loader and gives a deterministic snapshot per fixture. `fixtureFor(cat)` is a `switch` returning small typed values — one Tenant, one BaseModel, one GPUPool, one map[string][]GPUNode with a single key/item, etc.

- [ ] **Step 2: Write the snapshot test.**

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCSVSnapshots pins the canonical-column CSV output for every
// category against a small in-memory fixture. 12 categories are
// byte-identical to a stored snapshot; 7 categories carry
// intentionally-updated snapshots documenting the spec-mandated
// diffs (3 widened tenancy overrides, 4 grouped reordered to
// name-first).
func TestCSVSnapshots(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown {
			continue
		}
		t.Run(cat.String(), func(t *testing.T) {
			got := renderSnapshot(t, cat)
			path := filepath.Join("testdata", "snapshots", cat.String()+".csv")
			if shouldUpdate() {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read snapshot: %v (run with UPDATE_SNAPSHOTS=1 to seed)", err)
			}
			if string(want) != got {
				t.Errorf("%s csv changed (run UPDATE_SNAPSHOTS=1 if expected):\n--- want\n%s\n--- got\n%s", cat, want, got)
			}
		})
	}
}

func shouldUpdate() bool {
	return os.Getenv("UPDATE_SNAPSHOTS") == "1"
}
```

- [ ] **Step 3: Seed the snapshot files.**

```bash
UPDATE_SNAPSHOTS=1 go test ./internal/cli/ -run TestCSVSnapshots
```

This writes 19 CSV files under `internal/cli/testdata/snapshots/`. Review each:

- 12 should match what `toolkit get <cat> -o csv` produced **before** this refactor (capture pre-refactor outputs separately if you haven't already — or accept that this snapshot represents the post-refactor canonical and rely on Task 7's `tables_test.go` rewrites for the byte-identical claim).
- 7 should reflect the documented diffs:
  - `limit-tenancy-override.csv`, `console-property-tenancy-override.csv`, `property-tenancy-override.csv` — widened columns.
  - `gpu-node.csv`, `dedicated-ai-cluster.csv`, `imported-model.csv`, `model-artifact.csv` — name-first ordering.

- [ ] **Step 4: Run the test without the update flag.**

```bash
go test ./internal/cli/ -run TestCSVSnapshots -v
```

Expected: PASS for all 19 cases.

- [ ] **Step 5: Commit (separately from the snapshots if reviewer prefers).**

```bash
git add internal/cli/snapshot_test.go internal/cli/testdata/snapshots/
git commit -m "test(cli): pin toolkit get -o csv output per category

12 categories are byte-identical to today's output; 7 reflect the
spec's intentional diffs (3 widened tenancy overrides + 4 grouped
reordered).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 11: Final verification

- [ ] **Step 1: Run the full test suite + linters.**

```bash
go test ./... -count=1 -race
go vet ./...
gofmt -l . | grep -v vendor | (! grep .)  # exit 0 if no diff
```

If the repo has a `make test` or `make ci` target, use that.

- [ ] **Step 2: Run `toolkit get` against a real environment** (skip if no live dataset).

```bash
go build -o /tmp/toolkit ./cmd/toolkit
/tmp/toolkit get tenant -o table | head
/tmp/toolkit get gpunode -o csv | head
/tmp/toolkit get basemodel --columns name,display-name,context -o csv | head
/tmp/toolkit get gpupool --columns help
/tmp/toolkit get gpupool --columns bogus  # expect error
/tmp/toolkit get gpupool --columns name -o json  # expect mutex error
```

- [ ] **Step 3: Start the TUI** and click through the 19 categories. Headers and widths should look identical to pre-refactor (except ServiceTenancy's "Type" → "Environment", which is the one intentional TUI header change).

```bash
/tmp/toolkit
```

- [ ] **Step 4: Run GitNexus impact analysis** on the refactor as a whole, before any follow-up edits.

```bash
npx gitnexus analyze
```

- [ ] **Step 5: Confirm the spec section 6 (Migration) deletions all landed.**

```bash
test ! -f internal/ui/tui/headers.go
test ! -f internal/ui/tui/row_builders.go
grep -L 'tableFromSlice\|tableFromGrouped\|tenantTable\|gpuPoolTable' internal/cli/get.go
```

All three checks should produce no output.

- [ ] **Step 6: Final commit if any fixup edits were needed during verification.**

If clean, no commit needed — the prior tasks already produced the durable history.

---

## Out of scope

The following intentional CLI behavior changes are accepted per the spec; do not "fix" them in this refactor:

- Tenancy override default columns widening (Decision #9).
- Grouped category column reordering (Decision #4).
- Tenant `IDs` rendered via `strings.Join` for both surfaces (was `GetTenantID` in TUI).
- Alias canonical shape (1 row per category, with joined aliases) — CLI used to be 1 row per alias.
- ServiceTenancy TUI header bug fixed (Type → Environment).
- CSV/TSV headers stay uppercased in CLI; TUI uses Title Case (preserves both today).

Follow-up ideas (separate PRs, not in this plan):
- Spec section 7 risks notes a `+key` syntax for `--columns` (defaults + extra). YAGNI for now.
- Snapshot test could also pin `-o table` output, not just csv. Skipped to keep the snapshots stable across terminal width changes.
