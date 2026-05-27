# Canonical column set per category

**Date:** 2026-05-23
**Status:** Approved (pending user spec review)
**Scope:** Refactor — collapse the parallel column definitions in `internal/cli/get.go` (table renderers) and `internal/ui/tui/{headers.go,row_builders.go}` into a single canonical registry under `internal/columns/`.

## Problem

Today CLI table output and the TUI maintain independent column lists per category. They have drifted: BaseModel's TUI shows Display Name / DAC Shape / Context but the CLI table shows Vendor / Type / Flags; ImportedModel is the inverse (CLI has Vendor/Version, TUI drops both); tenancy-override CLI tables are unusably thin (TENANT|NAME) while the TUI shows Regions + Min/Max/Value. Adding a field to a category requires editing both surfaces with no compiler-level link between them.

## Goals

1. Single source of truth for what columns a category has.
2. CLI table and TUI render from that source.
3. Power-user override: `toolkit get <cat> --columns ...`.
4. Default CLI behavior preserves today's output for all categories except the three tenancy overrides, which are widened to match the TUI (deliberate fix; today's defaults are unusable).

## Non-goals

- Changing structured-output (`json`/`jsonl`/`yaml`) behavior. Those continue to emit the full struct — column projection in structured output is `jq` / `yq` territory.
- Changing MCP behavior. MCP tools return full typed structs.
- Changing the loader, models, or the encoding layer (`internal/cli/output/`).

## Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | One canonical set per category (option C of the brainstorm: single set + CLI `--columns`). | Avoids the per-column "is this essential?" debate that institutionalizes drift in a tier-based design. |
| 2 | New top-level package `internal/columns/`. | Consumed by both `internal/cli/` and `internal/ui/tui/`; doesn't belong in either. |
| 3 | Generic `Column[T]`, typed `Set[T]` / `GroupedSet[T]` per category. | Matches existing house style (`definitionTable[T models.Definition]`, `tenancyOverrideTable[T models.NamedItem]`). |
| 4 | Grouped categories use `GroupedColumn[T]` with `Render(key string, item T) string`; no separate `KeyColumn` field. Author orders the columns explicitly — the key column lives wherever it belongs (today's TUI puts it second; canonical follows TUI). | Today's CLI and TUI disagree on grouped column ordering (CLI: key-first; TUI: name-first). A separate `KeyColumn` would force one ordering for both. A unified `GroupedColumn[T]` lets the author pick — and we pick TUI's name-first ordering, preserving TUI visuals and changing CLI for 4 grouped categories instead. |
| 5 | Column identifier is an explicit `Key` field, kebab-case (`capacity-type`, `actual-size`). | Stable across Title rewording; predictable for CLI users. |
| 6 | CLI defaults mirror the TUI: empty `--columns` renders every column declared on the set. `--columns` is a pure narrowing tool. | No per-column curation needed — registry order is the canonical order, no second-tier hidden columns. The legacy `Default bool` field was dropped once CLI/TUI aligned (it would have been `true` everywhere). |
| 7 | TUI ratios stay on the canonical `Column`. | Alternative is a parallel TUI-only map — reintroduces drift. CLI ignores `Ratio` at render time. |
| 8 | CLI defaults preserve today's output for 7 of 19 categories byte-for-byte; 12 diff intentionally (see test plan #5 for the full breakdown). | Where CLI and TUI today disagreed on header text or column ordering, the canonical layer picks the TUI's convention — there's no neutral choice and committing to one source prevents future drift. |
| 9 | CLI defaults are *widened* for `LimitTenancyOverride`, `ConsolePropertyTenancyOverride`, `PropertyTenancyOverride` to match the TUI. | Today's tenancy-override CLI tables (TENANT\|NAME only) are unusable; this refactor fixes them. |
| 10 | `--columns` on `-o json\|jsonl\|yaml` is a hard error. | Silent ignore breeds confusion when users expect projection in JSON. |
| 11 | Big-bang migration in one PR; no parallel-paths feature flag. | Categories are independent, conversions are mechanical, snapshot tests pin behavior. |

## Architecture

### File layout

```
internal/columns/
  columns.go              // Column[T], Set[T], GroupedSet[T], Defaults/Resolve helpers
  registry.go             // domain.Category → set lookup; RenderTable fan-out
  tenant.go
  base_model.go
  imported_model.go       // GroupedSet
  model_artifact.go       // GroupedSet
  gpu_pool.go
  gpu_node.go             // GroupedSet
  dac.go                  // GroupedSet
  environment.go
  service_tenancy.go
  alias.go
  limit_definition.go
  definition.go           // generic over models.Definition (Console + Property)
  limit_regional_override.go
  regional_override.go    // generic over models.DefinitionOverride
  limit_tenancy_override.go
  tenancy_override.go     // generic over models.DefinitionOverride
```

### Types

```go
package columns

// Column is a column for a flat (non-grouped) category.
type Column[T any] struct {
    Title   string         // header text shown to humans ("Capacity Type")
    Key     string         // identifier for --columns ("capacity-type")
    Default bool           // included by CLI table when --columns is empty
    Ratio   float64        // TUI proportional width hint; CLI ignores
    Render  func(T) string // cell extractor
}

// GroupedColumn is a column for a grouped category. Render receives
// both the group key (e.g. tenant id, pool name) and the item, so
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

// GroupedSet is the canonical column list for a grouped category
// (loader returns map[string][]T). The author orders Columns
// explicitly; the canonical ordering for the toolkit follows the
// current TUI (item name first, group key second) so TUI visual
// output is preserved.
type GroupedSet[T any] struct {
    Columns []GroupedColumn[T]
}
```

### Registry adapter

```go
// In internal/columns/registry.go.
// RenderTable is the single entrypoint the CLI calls. It type-switches
// on the category, applies --columns selection, and produces
// headers+rows for the chosen encoding (table/csv/tsv).
//
// `items` must be the concrete payload for cat: a typed slice
// (e.g. []models.Tenant) for flat categories, or a typed grouped map
// (e.g. map[string][]models.GPUNode) for grouped categories. The
// type switch unwraps it; a mismatch returns an error rather than
// panicking. `selected` is the parsed --columns value (nil/empty
// means "use Default==true columns").
func RenderTable(cat domain.Category, items any, selected []string) (headers []string, rows [][]string, err error)
```

The TUI consumes the typed `Set[T]` / `GroupedSet[T]` directly through small adapters (`tuiColumns`, `tuiRows`) — it doesn't go through `RenderTable` because it needs `table.Column{Title, Width}` shape, not `([]string, [][]string)`. For grouped categories the TUI adapter iterates sorted keys (same as `tableFromGrouped` today) and renders each `GroupedColumn` via `c.Render(key, item)`.

## CLI integration

### Flag

```
--columns string   comma-separated column keys (default: category's Default columns).
                   Use `--columns help` to list valid keys for the chosen category.
```

### Behavior

- **Empty:** render columns where `Default==true`, in declared order.
- **Comma-separated keys:** render exactly those columns, in the supplied order. Tokens are trimmed of leading/trailing whitespace (`name, status` ≡ `name,status`); empty tokens (`name,,status`) are an error. Unknown keys → fail fast with stderr listing valid keys for that category.
- **`--columns help`:** print key / title / default-yes-no table for the category, exit 0. `help` is reserved — no canonical column may use it as a Key; enforced by the registry-consistency test.
- **Applies to `table`, `csv`, `tsv` only.** With `-o json|jsonl|yaml`, presence of `--columns` is a hard error: `--columns has no effect with -o <fmt>; remove the flag or switch to -o table/csv/tsv`.
- **`--no-headers`** unchanged.
- **Shell completion:** when the positional category arg is already typed, complete column keys from that category's set.

### Wiring

In `addGetCommand` (`internal/cli/get.go`):

```go
getCmd.Flags().StringVar(&columnsArg, "columns", "", "comma-separated column keys (default: category's defaults). Use --columns help to list.")
_ = getCmd.RegisterFlagCompletionFunc("columns", func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
    if len(args) < 1 {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    cat, err := domain.ParseCategory(args[0])
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    return columns.KeysFor(cat), cobra.ShellCompDirectiveNoFileComp
})
```

`runGet` parses `--columns` into `[]string`, validates mutual exclusion with structured formats, and threads `selected` down into `emitCategory`. The 12 `*Table` callbacks in `get.go` are deleted; `writeSlice` / `writeMapFlat` / `writeMapWithKey` now consult `columns.RenderTable` for the table/csv/tsv branches.

## TUI integration

`internal/ui/tui/headers.go` and `internal/ui/tui/row_builders.go` are deleted. Where the model currently looks up `headerDefinitions[cat]` and calls the per-category `*ToRow` function, it now calls one of two adapters:

```go
// in internal/ui/tui (or internal/columns/tui.go)
func tuiColumnsFlat[T any](s columns.Set[T], totalWidth int) []table.Column { ... }
func tuiRowsFlat[T any](s columns.Set[T], items []T) []table.Row { ... }

func tuiColumnsGrouped[T any](g columns.GroupedSet[T], totalWidth int) []table.Column { ... }
func tuiRowsGrouped[T any](g columns.GroupedSet[T], grouped map[string][]T) []table.Row { ... }
```

The TUI ignores `Column.Default` (shows all columns) and `Column.Key` (CLI-only). Ratios are converted to widths via `int(ratio * totalWidth)`, same math today's TUI does via `headerDefinitions`.

Visual output for the TUI does not change.

## Per-category column inventory

Each category gets `Default==true` for every column the current CLI table renders, plus the additional TUI columns as `Default==false`. The three tenancy-override categories are exceptions — see Decision #9.

This file does not enumerate every column for every category; concrete contents are in the implementation plan. The contract is:

1. For each category, union(today's CLI columns ∪ today's TUI columns) becomes the canonical set.
2. CLI defaults preserve today's CLI table content, except:
   - The three tenancy overrides widen to today's TUI columns (Decision #9).
   - For grouped categories, the canonical column **ordering** follows today's TUI (item name first, group key second) — so CLI grouped tables (`gpunode`, `dac`, `importedmodel`, `modelartifact`) get reordered. TUI ordering is unchanged.
3. TUI shows all canonical columns at their canonical ordering (= today's TUI behavior).
4. Ratios for shared columns carry over from `headerDefinitions`; ratios for new-to-canonical-but-old-to-CLI columns are assigned to keep per-set sums ≈ 1.0.

## Test plan

1. **Registry-consistency** (`internal/columns/registry_test.go`)
   - Every `domain.Category` (except `CategoryUnknown`) is registered.
   - Within each set, Keys are unique and non-empty; Titles are non-empty; no Key equals the reserved literal `help`; ≥1 column has `Default==true`.
   - Ratios sum to ~1.0 (±0.02) per set. Sum scope: for `Set[T]`, `sum(Columns[i].Ratio)`; for `GroupedSet[T]`, `KeyColumn.Ratio + sum(Columns[i].Ratio)`.

2. **Per-category renders** (`internal/columns/<category>_test.go`)
   - Table-driven: one fixture per category, assert exact cell strings for each column. Replaces today's assertions in `internal/cli/tables_test.go` and `internal/ui/tui/row_builders_test.go`.

3. **CLI `--columns` flag** (`internal/cli/get_test.go`)
   - Empty → Default columns in declared order (snapshot).
   - `--columns name,status` → exactly those, in order.
   - `--columns unknown` → non-zero exit, stderr enumerates valid keys.
   - `--columns help` → prints key/title/default table, exit 0.
   - `--columns name -o json` → mutual-exclusion error.
   - Completion func returns the right keys for a given category arg.

4. **TUI adapter** (`internal/ui/tui/...`)
   - For a representative flat category (BaseModel) and a representative grouped one (GPUNode), assert `tuiColumns*` produces today's `[]table.Column{Title, Width}` at a fixed total width.
   - Assert `tuiRows*` produces today's `table.Row` cells for a fixture model.

5. **Behavior-preservation snapshot** (`internal/cli/snapshot_test.go`)
   - Capture `toolkit get <cat> -o csv` for every category against a fixed test dataset.
   - 7 of 19 categories match byte-for-byte against a stored snapshot.
   - 12 categories diff intentionally — the snapshots reflect the new canonical output:
     - **Widened (3, Decision #9)** — `LimitTenancyOverride`, `ConsolePropertyTenancyOverride`, `PropertyTenancyOverride`: default columns grow from `[TENANT, NAME]` to the full TUI set.
     - **Grouped reordered to item-name-first (4, Decision #4)** — `ImportedModel`, `ModelArtifact`, `GPUNode`, `DedicatedAICluster`: NAME column moves to position 0; group key moves to position 1.
     - **Header label/ordering aligned with TUI (4)** — `Tenant` ("IDS" → "OCID"), `BaseModel` (FLAGS/STATUS order swap), `LimitDefinition` ("DEFAULT MIN/MAX" → "MIN/MAX"), `Environment` (TYPE/REGION/REALM → REALM/TYPE/REGION ordering). Cell content unchanged; titles/order follow the TUI's existing convention so the canonical layer doesn't pick arbitrarily.
     - **Structural shape change (1)** — `Alias`: CLI table/CSV switches from one-row-per-alias (`ALIAS,CATEGORY`) to one-row-per-category (`NAME,ALIASES`) matching the TUI. Affects table/csv/tsv. JSON shape becomes `[{name, aliases}, ...]` (richer than the legacy `[{alias, category}, ...]`).

### Out of scope for tests

- MCP / structured output: untouched.
- Loader: untouched.

## Migration

Single PR. Order of edits (mechanical):

1. Land `internal/columns/columns.go` and `registry.go` with empty sets and a stubbed `RenderTable` that returns `not registered`.
2. Add one category at a time (`tenant.go`, then the next, …), porting the CLI `*Table` body into `Render` closures and copying ratios + titles from `headerDefinitions`. Tests for that category move with it.
3. Once all 19 are registered, swap `internal/cli/get.go` to consult the registry and delete the per-category `*Table` functions.
4. Swap the TUI to use the adapter and delete `headers.go` and `row_builders.go`.
5. Add `--columns` flag wiring and tests.
6. Run snapshot test; resolve diffs (expect 3 intentional ones).

No feature flag, no dual paths.

## Risks

- **CLI behavior changes** are limited to 12 categories:
  - 3 widened tenancy overrides (Decision #9): scripts that parsed two columns now get more. CSV/TSV consumers using positional indexing should review.
  - 4 grouped categories (Decision #4) with reordered columns (key was first, now second): same risk for positional consumers.
  - 4 categories with header/order alignment to TUI (`Tenant`, `BaseModel`, `LimitDefinition`, `Environment`): cell content unchanged; scripts that grep header text or use positional indexing should review.
  - 1 structural shape change (`Alias`): table/CSV row semantics flip from one-per-alias to one-per-category. JSON shape becomes the richer `[{name, aliases}, ...]`. Legacy `aliasItem` JSON shape is preserved only in MCP's `list_aliases` for backward compatibility (see `internal/mcp/tools.go`).
  - The other 7 produce byte-identical output, pinned by the snapshot test.
- **TUI visual changes:** none. The canonical column ordering for grouped categories matches today's TUI, and `internal/ui/tui/table_utils.go` `getItemKey`/`statsColumns`/`computeNumericStats` continue to work without changes (they reference columns by Title text, which is preserved).
- **Generics-over-Category dispatch** in `RenderTable` is a type switch — adding a category later means editing that switch. Acceptable: same edit cost as today's `emitCategory` switch.
- **Ratio drift in TUI** for any category where this refactor reshuffles ratios. Mitigated by the "ratios sum to ~1.0" registry test and the TUI adapter tests pinning representative outputs.
