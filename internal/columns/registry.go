package columns

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jingle2008/toolkit/internal/domain"
)

// registryEntry bundles a category's per-call dispatch (render and
// render-for-export) and its precomputed metadata (keys, titles,
// ratio sum). All five are derived from the same Set or GroupedSet
// at package-init time via newFlatEntry / newGroupedEntry, so the public
// functions in this file cannot drift across categories.
type registryEntry struct {
	keys         []string
	titles       []string
	ratioSum     float64
	render       func(items any, selected []string) ([]string, [][]string, error)
	renderForExport func(items any, realm, region string, selected []string) ([]string, [][]string, error)
}

// newFlatEntry builds a registryEntry from a flat columns.Set. The
// captured Set drives both the precomputed metadata and the render
// closures, so a column reorder lands in every consumer at once.
func newFlatEntry[T any](s Set[T]) registryEntry {
	return registryEntry{
		keys:     s.Keys(),
		titles:   s.Titles(),
		ratioSum: s.RatioSum(),
		render: func(items any, selected []string) ([]string, [][]string, error) {
			return renderFlat(s, items, selected)
		},
		renderForExport: func(items any, realm, region string, selected []string) ([]string, [][]string, error) {
			return renderFlatForExport(s, items, realm, region, selected)
		},
	}
}

// newGroupedEntry is the grouped counterpart to newFlatEntry.
func newGroupedEntry[T any](g GroupedSet[T]) registryEntry {
	return registryEntry{
		keys:     g.Keys(),
		titles:   g.Titles(),
		ratioSum: g.RatioSum(),
		render: func(items any, selected []string) ([]string, [][]string, error) {
			return renderGrouped(g, items, selected)
		},
		renderForExport: func(items any, realm, region string, selected []string) ([]string, [][]string, error) {
			return renderGroupedForExport(g, items, realm, region, selected)
		},
	}
}

// registry is the single per-category dispatch table the public
// functions in this file consume. Adding a new list-view category
// requires exactly one entry here; missing entries surface as
// "not registered" errors from RenderTable.
var registry = map[domain.Category]registryEntry{
	domain.Tenant:                          newFlatEntry(TenantColumns),
	domain.Alias:                           newFlatEntry(AliasColumns),
	domain.Environment:                     newFlatEntry(EnvironmentColumns),
	domain.ServiceTenancy:                  newFlatEntry(ServiceTenancyColumns),
	domain.LimitDefinition:                 newFlatEntry(LimitDefinitionColumns),
	domain.LimitRegionalOverride:           newFlatEntry(LimitRegionalOverrideColumns),
	domain.BaseModel:                       newFlatEntry(BaseModelColumns),
	domain.GPUPool:                         newFlatEntry(GPUPoolColumns),
	domain.ConsolePropertyDefinition:       newFlatEntry(ConsolePropertyDefinitionColumns),
	domain.PropertyDefinition:              newFlatEntry(PropertyDefinitionColumns),
	domain.ConsolePropertyRegionalOverride: newFlatEntry(ConsolePropertyRegionalOverrideColumns),
	domain.PropertyRegionalOverride:        newFlatEntry(PropertyRegionalOverrideColumns),
	domain.GPUNode:                         newGroupedEntry(GPUNodeColumns),
	domain.DedicatedAICluster:              newGroupedEntry(DACColumns),
	domain.ImportedModel:                   newGroupedEntry(ImportedModelColumns),
	domain.ModelArtifact:                   newGroupedEntry(ModelArtifactColumns),
	domain.LimitTenancyOverride:            newGroupedEntry(LimitTenancyOverrideColumns),
	domain.ConsolePropertyTenancyOverride:  newGroupedEntry(ConsolePropertyTenancyOverrideColumns),
	domain.PropertyTenancyOverride:         newGroupedEntry(PropertyTenancyOverrideColumns),
}

// IsRegistered reports whether cat has a canonical column set.
func IsRegistered(cat domain.Category) bool {
	_, ok := registry[cat]
	return ok
}

// KeysFor returns the declared keys for cat in order.
// Returns nil for unregistered categories.
func KeysFor(cat domain.Category) []string {
	if e, ok := registry[cat]; ok {
		return e.keys
	}
	return nil
}

// RatioSumFor returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSumFor(cat domain.Category) float64 {
	if e, ok := registry[cat]; ok {
		return e.ratioSum
	}
	return 0
}

// TitlesFor returns the Title for each column of cat in declared order.
func TitlesFor(cat domain.Category) []string {
	if e, ok := registry[cat]; ok {
		return e.titles
	}
	return nil
}

// RenderTable is the single entrypoint the CLI calls. It dispatches
// on cat via the registry, applies --columns selection, and produces
// headers+rows for the chosen encoding (table/csv/tsv). headers are
// uppercased to preserve today's CLI table headers (NAME, STATUS,
// ...); the TUI adapter (in internal/ui/tui) uses Titles as-is.
//
// `items` must be the concrete payload for cat. `selected` is the
// parsed --columns list (empty means "render every column").
func RenderTable(cat domain.Category, items any, selected []string) ([]string, [][]string, error) {
	if e, ok := registry[cat]; ok {
		return e.render(items, selected)
	}
	return nil, nil, fmt.Errorf("category %s is not registered with the columns package", cat)
}

// RenderTableForExport mirrors RenderTable but consults each
// column's RenderForExport when present, producing the values
// downstream OCI tooling expects (e.g. fully-qualified OCIDs in
// place of raw Name suffixes for DAC and ImportedModel rows).
// Columns without RenderForExport fall back to Render, so categories
// that have nothing export-specific to say behave identically to
// RenderTable.
//
// When either realm or region is empty the function short-circuits
// to RenderTable — RenderForExport closures typically format both into
// the output, and a partial env would produce malformed OCIDs like
// `ocid1.<type>.oc1..suffix` (missing region) or
// `ocid1.<type>..iad.suffix` (missing realm). Callers without a
// fully-populated env (e.g. unit tests that exercise the column
// registry directly) get raw display-mode output.
func RenderTableForExport(cat domain.Category, items any, realm, region string, selected []string) ([]string, [][]string, error) {
	if realm == "" || region == "" {
		return RenderTable(cat, items, selected)
	}
	if e, ok := registry[cat]; ok {
		return e.renderForExport(items, realm, region, selected)
	}
	return nil, nil, fmt.Errorf("category %s is not registered with the columns package", cat)
}

// HelpTable returns a (Key, Title) row per column of cat,
// for the `--columns help` output. Empty if cat is unregistered.
func HelpTable(cat domain.Category) (headers []string, rows [][]string) {
	keys := KeysFor(cat)
	if keys == nil {
		return nil, nil
	}
	titles := TitlesFor(cat)
	headers = []string{"KEY", "TITLE"}
	rows = make([][]string, len(keys))
	for i, k := range keys {
		rows[i] = []string{k, titles[i]}
	}
	return headers, rows
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

// renderFlat renders a flat category for RenderTable. It picks
// defaults vs. selected, then runs each column's Render against
// each item.
func renderFlat[T any](s Set[T], items any, selected []string) ([]string, [][]string, error) {
	typed, ok := items.([]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderFlat: items has wrong type %T", items)
	}
	cols, err := selectFlat(s, selected)
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

func selectFlat[T any](s Set[T], selected []string) ([]Column[T], error) {
	if len(selected) == 0 {
		return s.Columns, nil
	}
	return s.Select(selected)
}

// renderGrouped renders a grouped category for RenderTable.
func renderGrouped[T any](g GroupedSet[T], items any, selected []string) ([]string, [][]string, error) {
	typed, ok := items.(map[string][]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderGrouped: items has wrong type %T", items)
	}
	cols, err := selectGrouped(g, selected)
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

func selectGrouped[T any](g GroupedSet[T], selected []string) ([]GroupedColumn[T], error) {
	if len(selected) == 0 {
		return g.Columns, nil
	}
	return g.Select(selected)
}

// renderFlatForExport mirrors renderFlat but uses Column.RenderForExport
// when set, falling back to Column.Render otherwise.
func renderFlatForExport[T any](s Set[T], items any, realm, region string, selected []string) ([]string, [][]string, error) {
	typed, ok := items.([]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderFlatForExport: items has wrong type %T", items)
	}
	cols, err := selectFlat(s, selected)
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
			if c.RenderForExport != nil {
				row[j] = c.RenderForExport(realm, region, it)
			} else {
				row[j] = c.Render(it)
			}
		}
		rows[i] = row
	}
	return headers, rows, nil
}

// renderGroupedForExport mirrors renderGrouped but uses
// GroupedColumn.RenderForExport when set.
func renderGroupedForExport[T any](g GroupedSet[T], items any, realm, region string, selected []string) ([]string, [][]string, error) {
	typed, ok := items.(map[string][]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderGroupedForExport: items has wrong type %T", items)
	}
	cols, err := selectGrouped(g, selected)
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
				if c.RenderForExport != nil {
					row[j] = c.RenderForExport(realm, region, k, it)
				} else {
					row[j] = c.Render(k, it)
				}
			}
			rows = append(rows, row)
		}
	}
	return headers, rows, nil
}
