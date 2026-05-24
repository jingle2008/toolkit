package columns

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jingle2008/toolkit/internal/domain"
)

// IsRegistered reports whether cat has a canonical column set.
// Implementation lives alongside the per-category files; this
// switch is the single edit-site when a new category is added.
func IsRegistered(cat domain.Category) bool {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return true
	case domain.Alias:
		return true
	case domain.Environment:
		return true
	case domain.ServiceTenancy:
		return true
	case domain.LimitDefinition:
		return true
	case domain.LimitRegionalOverride:
		return true
	}
	return false
}

// KeysFor returns the declared keys for cat in order.
// Returns nil for unregistered categories.
func KeysFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return TenantColumns.Keys()
	case domain.Alias:
		return AliasColumns.Keys()
	case domain.Environment:
		return EnvironmentColumns.Keys()
	case domain.ServiceTenancy:
		return ServiceTenancyColumns.Keys()
	case domain.LimitDefinition:
		return LimitDefinitionColumns.Keys()
	case domain.LimitRegionalOverride:
		return LimitRegionalOverrideColumns.Keys()
	}
	return nil
}

// DefaultsFor returns the Default flag for each column of cat in
// declared order. The two slices KeysFor / DefaultsFor share the
// same indices; together they're enough to drive shell completion
// and the `--columns help` table.
func DefaultsFor(cat domain.Category) []bool {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		out := make([]bool, len(TenantColumns.Columns))
		for i, c := range TenantColumns.Columns {
			out[i] = c.Default
		}
		return out
	case domain.Alias:
		out := make([]bool, len(AliasColumns.Columns))
		for i, c := range AliasColumns.Columns {
			out[i] = c.Default
		}
		return out
	case domain.Environment:
		out := make([]bool, len(EnvironmentColumns.Columns))
		for i, c := range EnvironmentColumns.Columns {
			out[i] = c.Default
		}
		return out
	case domain.ServiceTenancy:
		out := make([]bool, len(ServiceTenancyColumns.Columns))
		for i, c := range ServiceTenancyColumns.Columns {
			out[i] = c.Default
		}
		return out
	case domain.LimitDefinition:
		out := make([]bool, len(LimitDefinitionColumns.Columns))
		for i, c := range LimitDefinitionColumns.Columns {
			out[i] = c.Default
		}
		return out
	case domain.LimitRegionalOverride:
		out := make([]bool, len(LimitRegionalOverrideColumns.Columns))
		for i, c := range LimitRegionalOverrideColumns.Columns {
			out[i] = c.Default
		}
		return out
	}
	return nil
}

// RatioSum returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSum(cat domain.Category) float64 {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		var sum float64
		for _, c := range TenantColumns.Columns {
			sum += c.Ratio
		}
		return sum
	case domain.Alias:
		var sum float64
		for _, c := range AliasColumns.Columns {
			sum += c.Ratio
		}
		return sum
	case domain.Environment:
		var sum float64
		for _, c := range EnvironmentColumns.Columns {
			sum += c.Ratio
		}
		return sum
	case domain.ServiceTenancy:
		var sum float64
		for _, c := range ServiceTenancyColumns.Columns {
			sum += c.Ratio
		}
		return sum
	case domain.LimitDefinition:
		var sum float64
		for _, c := range LimitDefinitionColumns.Columns {
			sum += c.Ratio
		}
		return sum
	case domain.LimitRegionalOverride:
		var sum float64
		for _, c := range LimitRegionalOverrideColumns.Columns {
			sum += c.Ratio
		}
		return sum
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
	case domain.Tenant:
		return renderFlat(TenantColumns, items, selected)
	case domain.Alias:
		return renderFlat(AliasColumns, items, selected)
	case domain.Environment:
		return renderFlat(EnvironmentColumns, items, selected)
	case domain.ServiceTenancy:
		return renderFlat(ServiceTenancyColumns, items, selected)
	case domain.LimitDefinition:
		return renderFlat(LimitDefinitionColumns, items, selected)
	case domain.LimitRegionalOverride:
		return renderFlat(LimitRegionalOverrideColumns, items, selected)
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
	case domain.Tenant:
		out := make([]string, len(TenantColumns.Columns))
		for i, c := range TenantColumns.Columns {
			out[i] = c.Title
		}
		return out
	case domain.Alias:
		out := make([]string, len(AliasColumns.Columns))
		for i, c := range AliasColumns.Columns {
			out[i] = c.Title
		}
		return out
	case domain.Environment:
		out := make([]string, len(EnvironmentColumns.Columns))
		for i, c := range EnvironmentColumns.Columns {
			out[i] = c.Title
		}
		return out
	case domain.ServiceTenancy:
		out := make([]string, len(ServiceTenancyColumns.Columns))
		for i, c := range ServiceTenancyColumns.Columns {
			out[i] = c.Title
		}
		return out
	case domain.LimitDefinition:
		out := make([]string, len(LimitDefinitionColumns.Columns))
		for i, c := range LimitDefinitionColumns.Columns {
			out[i] = c.Title
		}
		return out
	case domain.LimitRegionalOverride:
		out := make([]string, len(LimitRegionalOverrideColumns.Columns))
		for i, c := range LimitRegionalOverrideColumns.Columns {
			out[i] = c.Title
		}
		return out
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

