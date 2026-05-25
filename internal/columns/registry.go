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
	case domain.BaseModel:
		return true
	case domain.GpuPool:
		return true
	case domain.ConsolePropertyDefinition:
		return true
	case domain.PropertyDefinition:
		return true
	case domain.ConsolePropertyRegionalOverride:
		return true
	case domain.PropertyRegionalOverride:
		return true
	case domain.GpuNode:
		return true
	case domain.DedicatedAICluster:
		return true
	case domain.ImportedModel:
		return true
	case domain.ModelArtifact:
		return true
	case domain.LimitTenancyOverride:
		return true
	case domain.ConsolePropertyTenancyOverride:
		return true
	case domain.PropertyTenancyOverride:
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
	case domain.BaseModel:
		return BaseModelColumns.Keys()
	case domain.GpuPool:
		return GpuPoolColumns.Keys()
	case domain.ConsolePropertyDefinition:
		return ConsolePropertyDefinitionColumns.Keys()
	case domain.PropertyDefinition:
		return PropertyDefinitionColumns.Keys()
	case domain.ConsolePropertyRegionalOverride:
		return ConsolePropertyRegionalOverrideColumns.Keys()
	case domain.PropertyRegionalOverride:
		return PropertyRegionalOverrideColumns.Keys()
	case domain.GpuNode:
		return GpuNodeColumns.Keys()
	case domain.DedicatedAICluster:
		return DacColumns.Keys()
	case domain.ImportedModel:
		return ImportedModelColumns.Keys()
	case domain.ModelArtifact:
		return ModelArtifactColumns.Keys()
	case domain.LimitTenancyOverride:
		return LimitTenancyOverrideColumns.Keys()
	case domain.ConsolePropertyTenancyOverride:
		return ConsolePropertyTenancyOverrideColumns.Keys()
	case domain.PropertyTenancyOverride:
		return PropertyTenancyOverrideColumns.Keys()
	}
	return nil
}

// RatioSum returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSum(cat domain.Category) float64 {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return TenantColumns.RatioSum()
	case domain.Alias:
		return AliasColumns.RatioSum()
	case domain.Environment:
		return EnvironmentColumns.RatioSum()
	case domain.ServiceTenancy:
		return ServiceTenancyColumns.RatioSum()
	case domain.LimitDefinition:
		return LimitDefinitionColumns.RatioSum()
	case domain.LimitRegionalOverride:
		return LimitRegionalOverrideColumns.RatioSum()
	case domain.BaseModel:
		return BaseModelColumns.RatioSum()
	case domain.GpuPool:
		return GpuPoolColumns.RatioSum()
	case domain.ConsolePropertyDefinition:
		return ConsolePropertyDefinitionColumns.RatioSum()
	case domain.PropertyDefinition:
		return PropertyDefinitionColumns.RatioSum()
	case domain.ConsolePropertyRegionalOverride:
		return ConsolePropertyRegionalOverrideColumns.RatioSum()
	case domain.PropertyRegionalOverride:
		return PropertyRegionalOverrideColumns.RatioSum()
	case domain.GpuNode:
		return GpuNodeColumns.RatioSum()
	case domain.DedicatedAICluster:
		return DacColumns.RatioSum()
	case domain.ImportedModel:
		return ImportedModelColumns.RatioSum()
	case domain.ModelArtifact:
		return ModelArtifactColumns.RatioSum()
	case domain.LimitTenancyOverride:
		return LimitTenancyOverrideColumns.RatioSum()
	case domain.ConsolePropertyTenancyOverride:
		return ConsolePropertyTenancyOverrideColumns.RatioSum()
	case domain.PropertyTenancyOverride:
		return PropertyTenancyOverrideColumns.RatioSum()
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
// parsed --columns list (empty means "render every column").
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
	case domain.BaseModel:
		return renderFlat(BaseModelColumns, items, selected)
	case domain.GpuPool:
		return renderFlat(GpuPoolColumns, items, selected)
	case domain.ConsolePropertyDefinition:
		return renderFlat(ConsolePropertyDefinitionColumns, items, selected)
	case domain.PropertyDefinition:
		return renderFlat(PropertyDefinitionColumns, items, selected)
	case domain.ConsolePropertyRegionalOverride:
		return renderFlat(ConsolePropertyRegionalOverrideColumns, items, selected)
	case domain.PropertyRegionalOverride:
		return renderFlat(PropertyRegionalOverrideColumns, items, selected)
	case domain.GpuNode:
		return renderGrouped(GpuNodeColumns, items, selected)
	case domain.DedicatedAICluster:
		return renderGrouped(DacColumns, items, selected)
	case domain.ImportedModel:
		return renderGrouped(ImportedModelColumns, items, selected)
	case domain.ModelArtifact:
		return renderGrouped(ModelArtifactColumns, items, selected)
	case domain.LimitTenancyOverride:
		return renderGrouped(LimitTenancyOverrideColumns, items, selected)
	case domain.ConsolePropertyTenancyOverride:
		return renderGrouped(ConsolePropertyTenancyOverrideColumns, items, selected)
	case domain.PropertyTenancyOverride:
		return renderGrouped(PropertyTenancyOverrideColumns, items, selected)
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
//
//nolint:cyclop // a per-category switch is the contract here
func RenderTableForExport(cat domain.Category, items any, realm, region string, selected []string) ([]string, [][]string, error) {
	if realm == "" || region == "" {
		return RenderTable(cat, items, selected)
	}
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return renderFlatExport(TenantColumns, items, realm, region, selected)
	case domain.Alias:
		return renderFlatExport(AliasColumns, items, realm, region, selected)
	case domain.Environment:
		return renderFlatExport(EnvironmentColumns, items, realm, region, selected)
	case domain.ServiceTenancy:
		return renderFlatExport(ServiceTenancyColumns, items, realm, region, selected)
	case domain.LimitDefinition:
		return renderFlatExport(LimitDefinitionColumns, items, realm, region, selected)
	case domain.LimitRegionalOverride:
		return renderFlatExport(LimitRegionalOverrideColumns, items, realm, region, selected)
	case domain.BaseModel:
		return renderFlatExport(BaseModelColumns, items, realm, region, selected)
	case domain.GpuPool:
		return renderFlatExport(GpuPoolColumns, items, realm, region, selected)
	case domain.ConsolePropertyDefinition:
		return renderFlatExport(ConsolePropertyDefinitionColumns, items, realm, region, selected)
	case domain.PropertyDefinition:
		return renderFlatExport(PropertyDefinitionColumns, items, realm, region, selected)
	case domain.ConsolePropertyRegionalOverride:
		return renderFlatExport(ConsolePropertyRegionalOverrideColumns, items, realm, region, selected)
	case domain.PropertyRegionalOverride:
		return renderFlatExport(PropertyRegionalOverrideColumns, items, realm, region, selected)
	case domain.GpuNode:
		return renderGroupedExport(GpuNodeColumns, items, realm, region, selected)
	case domain.DedicatedAICluster:
		return renderGroupedExport(DacColumns, items, realm, region, selected)
	case domain.ImportedModel:
		return renderGroupedExport(ImportedModelColumns, items, realm, region, selected)
	case domain.ModelArtifact:
		return renderGroupedExport(ModelArtifactColumns, items, realm, region, selected)
	case domain.LimitTenancyOverride:
		return renderGroupedExport(LimitTenancyOverrideColumns, items, realm, region, selected)
	case domain.ConsolePropertyTenancyOverride:
		return renderGroupedExport(ConsolePropertyTenancyOverrideColumns, items, realm, region, selected)
	case domain.PropertyTenancyOverride:
		return renderGroupedExport(PropertyTenancyOverrideColumns, items, realm, region, selected)
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

// TitlesFor returns the Title for each column of cat in declared order.
func TitlesFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return TenantColumns.Titles()
	case domain.Alias:
		return AliasColumns.Titles()
	case domain.Environment:
		return EnvironmentColumns.Titles()
	case domain.ServiceTenancy:
		return ServiceTenancyColumns.Titles()
	case domain.LimitDefinition:
		return LimitDefinitionColumns.Titles()
	case domain.LimitRegionalOverride:
		return LimitRegionalOverrideColumns.Titles()
	case domain.BaseModel:
		return BaseModelColumns.Titles()
	case domain.GpuPool:
		return GpuPoolColumns.Titles()
	case domain.ConsolePropertyDefinition:
		return ConsolePropertyDefinitionColumns.Titles()
	case domain.PropertyDefinition:
		return PropertyDefinitionColumns.Titles()
	case domain.ConsolePropertyRegionalOverride:
		return ConsolePropertyRegionalOverrideColumns.Titles()
	case domain.PropertyRegionalOverride:
		return PropertyRegionalOverrideColumns.Titles()
	case domain.GpuNode:
		return GpuNodeColumns.Titles()
	case domain.DedicatedAICluster:
		return DacColumns.Titles()
	case domain.ImportedModel:
		return ImportedModelColumns.Titles()
	case domain.ModelArtifact:
		return ModelArtifactColumns.Titles()
	case domain.LimitTenancyOverride:
		return LimitTenancyOverrideColumns.Titles()
	case domain.ConsolePropertyTenancyOverride:
		return ConsolePropertyTenancyOverrideColumns.Titles()
	case domain.PropertyTenancyOverride:
		return PropertyTenancyOverrideColumns.Titles()
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
		return s.Columns, nil
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
		return g.Columns, nil
	}
	return g.SelectColumns(selected)
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
