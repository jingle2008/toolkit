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
// at package-init time via flatEntry / groupedEntry, so the public
// functions in this file cannot drift across categories.
type registryEntry struct {
	keys         []string
	titles       []string
	ratioSum     float64
	render       func(items any, selected []string) ([]string, [][]string, error)
	renderExport func(items any, realm, region string, selected []string) ([]string, [][]string, error)
}

// flatEntry builds a registryEntry from a flat columns.Set. The
// captured Set drives both the precomputed metadata and the render
// closures, so a column reorder lands in every consumer at once.
func flatEntry[T any](s Set[T]) registryEntry {
	return registryEntry{
		keys:     s.Keys(),
		titles:   s.Titles(),
		ratioSum: s.RatioSum(),
		render: func(items any, selected []string) ([]string, [][]string, error) {
			return renderFlat(s, items, selected)
		},
		renderExport: func(items any, realm, region string, selected []string) ([]string, [][]string, error) {
			return renderFlatExport(s, items, realm, region, selected)
		},
	}
}

// groupedEntry is the grouped counterpart to flatEntry.
func groupedEntry[T any](g GroupedSet[T]) registryEntry {
	return registryEntry{
		keys:     g.Keys(),
		titles:   g.Titles(),
		ratioSum: g.RatioSum(),
		render: func(items any, selected []string) ([]string, [][]string, error) {
			return renderGrouped(g, items, selected)
		},
		renderExport: func(items any, realm, region string, selected []string) ([]string, [][]string, error) {
			return renderGroupedExport(g, items, realm, region, selected)
		},
	}
}

// registry is the single per-category dispatch table the public
// functions in this file consume. Adding a new list-view category
// requires exactly one entry here; missing entries surface as
// "not registered" errors from RenderTable.
var registry = map[domain.Category]registryEntry{
	domain.Tenant:                          flatEntry(TenantColumns),
	domain.Alias:                           flatEntry(AliasColumns),
	domain.Environment:                     flatEntry(EnvironmentColumns),
	domain.ServiceTenancy:                  flatEntry(ServiceTenancyColumns),
	domain.LimitDefinition:                 flatEntry(LimitDefinitionColumns),
	domain.LimitRegionalOverride:           flatEntry(LimitRegionalOverrideColumns),
	domain.BaseModel:                       flatEntry(BaseModelColumns),
	domain.GpuPool:                         flatEntry(GpuPoolColumns),
	domain.ConsolePropertyDefinition:       flatEntry(ConsolePropertyDefinitionColumns),
	domain.PropertyDefinition:              flatEntry(PropertyDefinitionColumns),
	domain.ConsolePropertyRegionalOverride: flatEntry(ConsolePropertyRegionalOverrideColumns),
	domain.PropertyRegionalOverride:        flatEntry(PropertyRegionalOverrideColumns),
	domain.GpuNode:                         groupedEntry(GpuNodeColumns),
	domain.DedicatedAICluster:              groupedEntry(DacColumns),
	domain.ImportedModel:                   groupedEntry(ImportedModelColumns),
	domain.ModelArtifact:                   groupedEntry(ModelArtifactColumns),
	domain.LimitTenancyOverride:            groupedEntry(LimitTenancyOverrideColumns),
	domain.ConsolePropertyTenancyOverride:  groupedEntry(ConsolePropertyTenancyOverrideColumns),
	domain.PropertyTenancyOverride:         groupedEntry(PropertyTenancyOverrideColumns),
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

// RatioSum returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSum(cat domain.Category) float64 {
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
// column's ExportRender when present, producing the values
// downstream OCI tooling expects (e.g. fully-qualified OCIDs in
// place of raw Name suffixes for DAC and ImportedModel rows).
// Columns without ExportRender fall back to Render, so categories
// that have nothing export-specific to say behave identically to
// RenderTable.
//
// When either realm or region is empty the function short-circuits
// to RenderTable — ExportRender closures typically format both into
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
		return e.renderExport(items, realm, region, selected)
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
		return g.Columns, nil
	}
	return g.Select(selected)
}

// renderFlatExport mirrors renderFlat but uses Column.ExportRender
// when set, falling back to Column.Render otherwise.
func renderFlatExport[T any](s Set[T], items any, realm, region string, selected []string) ([]string, [][]string, error) {
	typed, ok := items.([]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderFlatExport: items has wrong type %T", items)
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
			if c.ExportRender != nil {
				row[j] = c.ExportRender(realm, region, it)
			} else {
				row[j] = c.Render(it)
			}
		}
		rows[i] = row
	}
	return headers, rows, nil
}

// renderGroupedExport mirrors renderGrouped but uses
// GroupedColumn.ExportRender when set.
func renderGroupedExport[T any](g GroupedSet[T], items any, realm, region string, selected []string) ([]string, [][]string, error) {
	typed, ok := items.(map[string][]T)
	if !ok {
		return nil, nil, fmt.Errorf("renderGroupedExport: items has wrong type %T", items)
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
				if c.ExportRender != nil {
					row[j] = c.ExportRender(realm, region, k, it)
				} else {
					row[j] = c.Render(k, it)
				}
			}
			rows = append(rows, row)
		}
	}
	return headers, rows, nil
}
