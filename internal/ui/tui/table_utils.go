package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

type header struct {
	text           string
	ratio          float64
	truncateMiddle bool
}

type tableStats map[string]int

var statsColumns = map[domain.Category][]string{
	domain.GpuPool:            {common.SizeCol, "GPUs"},
	domain.GpuNode:            {"Total", common.FreeCol},
	domain.DedicatedAICluster: {common.SizeCol},
}

func faultyPred[T models.Faulty](t T) bool {
	return t.IsFaulty()
}

func headersFromSet[T any](cols []columns.Column[T]) []header {
	out := make([]header, len(cols))
	for i, c := range cols {
		out[i] = header{text: c.Title, ratio: c.Ratio, truncateMiddle: c.TruncateMiddle}
	}
	return out
}

func headersFromGroupedSet[T any](cols []columns.GroupedColumn[T]) []header {
	out := make([]header, len(cols))
	for i, c := range cols {
		out[i] = header{text: c.Title, ratio: c.Ratio, truncateMiddle: c.TruncateMiddle}
	}
	return out
}

/*
getHeaders returns the header definitions for a given category.
If no headers are defined for the category, it returns nil.
*/
func getHeaders(category domain.Category) []header {
	switch category { //nolint:exhaustive
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
	case domain.GpuPool:
		return headersFromSet(columns.GpuPoolColumns.Columns)
	case domain.Alias:
		return headersFromSet(columns.AliasColumns.Columns)
	case domain.GpuNode:
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

/*
getTableRows returns the table rows for a given category, using the appropriate handler.
If the context is not valid for the category, it is set to nil.
Returns: rows, stats (nil if not applicable)
*/
func getTableRows(dataset *models.Dataset, category domain.Category, context *domain.ToolkitContext, filter string, sortColumn string, sortAsc bool, faultyOnly bool) ([]table.Row, tableStats) {
	if context != nil && !context.Category.IsScopeOf(category) {
		context = nil
	}

	src, exists := rowSources[category]
	if !exists {
		return nil, nil
	}
	rows := src(rowCtx{
		dataset: dataset,
		context: context,
		filter:  filter,
		faulty:  faultyOnly,
	})
	if sortColumn != "" && len(rows) > 0 {
		headers := getHeaders(category)
		sortRows(rows, headers, sortColumn, sortAsc)
	}
	return rows, computeStats(category, rows)
}

// computeStats calculates stats for the given category and rows.
func computeStats(category domain.Category, rows []table.Row) tableStats {
	if len(rows) == 0 {
		return nil
	}

	stats := computeNumericStats(category, rows)
	if category == domain.DedicatedAICluster {
		stats = appendDedicatedAIClusterStats(rows, stats)
	}

	return stats
}

// computeNumericStats sums numeric columns defined for the category.
func computeNumericStats(category domain.Category, rows []table.Row) tableStats {
	cols, ok := statsColumns[category]
	if !ok || len(rows) == 0 {
		return nil
	}

	headers := getHeaders(category)
	idx := make(map[string]int)
	for i, h := range headers {
		idx[h.text] = i
	}

	totals := make(tableStats)
	for _, col := range cols {
		columnIdx, ok := idx[col]
		if !ok {
			return nil // header missing, bail out
		}
		sum := 0
		for _, r := range rows {
			v, err := strconv.Atoi(r[columnIdx])
			if err == nil {
				sum += v
			}
		}
		totals[col] = sum
	}

	return totals
}

func appendDedicatedAIClusterStats(rows []table.Row, stats tableStats) tableStats {
	headers := getHeaders(domain.DedicatedAICluster)
	statusIdx := -1
	for i, h := range headers {
		if h.text == "Status" {
			statusIdx = i
			break
		}
	}

	var active, failed int
	for _, r := range rows {
		switch strings.ToLower(strings.TrimSpace(r[statusIdx])) {
		case "active", "ready":
			active++
		case "fail", "failed":
			failed++
		}
	}

	stats["Active"] = active
	stats["Failed"] = failed
	return stats
}

// filterRowsScoped is used for tenancy and other scoped overrides.
// Accepts a Logger interface for decoupling from zap.
func filterRowsScoped[T models.NamedFilterable](
	g map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
	faultyOnly bool,
	rowFn func(T, string) table.Row,
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
	matches := collections.FilterMap(g, key, name, filter, pred)
	results := make([]table.Row, 0, len(matches))
	for key, m := range matches {
		for _, v := range m {
			results = append(results, rowFn(v, key))
		}
	}
	return results
}

/*
getItemKey returns the ItemKey for a given category and table row.
*/
func getItemKey(category domain.Category, row table.Row) models.ItemKey {
	if len(row) == 0 {
		return nil
	}
	switch category {
	case domain.Tenant, domain.LimitDefinition, domain.Environment, domain.ServiceTenancy,
		domain.ConsolePropertyDefinition, domain.PropertyDefinition, domain.GpuPool,
		domain.LimitRegionalOverride, domain.ConsolePropertyRegionalOverride,
		domain.PropertyRegionalOverride, domain.ModelArtifact, domain.Alias, domain.BaseModel:
		return row[0]
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GpuNode, domain.DedicatedAICluster,
		domain.ImportedModel:
		return models.ScopedItemKey{Scope: row[1], Name: row[0]}
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

/*
findItem returns the item from the dataset for a given category and key.
*/
//nolint:cyclop
func findItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) any {
	if key == nil {
		return nil
	}
	switch category {
	case domain.Tenant:
		return findTenant(dataset, key)
	case domain.LimitDefinition:
		return findLimitDefinition(dataset, key)
	case domain.ConsolePropertyDefinition:
		return findConsolePropertyDefinition(dataset, key)
	case domain.PropertyDefinition:
		return findPropertyDefinition(dataset, key)
	case domain.LimitTenancyOverride:
		return findLimitTenancyOverride(dataset, key)
	case domain.ConsolePropertyTenancyOverride:
		return findConsolePropertyTenancyOverride(dataset, key)
	case domain.PropertyTenancyOverride:
		return findPropertyTenancyOverride(dataset, key)
	case domain.LimitRegionalOverride:
		return findLimitRegionalOverride(dataset, key)
	case domain.ConsolePropertyRegionalOverride:
		return findConsolePropertyRegionalOverride(dataset, key)
	case domain.PropertyRegionalOverride:
		return findPropertyRegionalOverride(dataset, key)
	case domain.BaseModel:
		return findBaseModel(dataset, key)
	case domain.ImportedModel:
		return findImportedModel(dataset, key)
	case domain.ModelArtifact:
		return findModelArtifact(dataset, key)
	case domain.Environment:
		return findEnvironment(dataset, key)
	case domain.ServiceTenancy:
		return findServiceTenancy(dataset, key)
	case domain.GpuPool:
		return findGpuPool(dataset, key)
	case domain.GpuNode:
		return findGpuNode(dataset, key)
	case domain.DedicatedAICluster:
		return findDedicatedAICluster(dataset, key)
	case domain.Alias:
		return key
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

func findTenant(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.Tenants, key.(string))
}

func findLimitDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.LimitDefinitionGroup.Values, key.(string))
}

func findConsolePropertyDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ConsolePropertyDefinitionGroup.Values, key.(string))
}

func findPropertyDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.PropertyDefinitionGroup.Values, key.(string))
}

func findLimitTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.LimitTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findConsolePropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.ConsolePropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findPropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.PropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findLimitRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.LimitRegionalOverrides, key.(string))
}

func findConsolePropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ConsolePropertyRegionalOverrides, key.(string))
}

func findPropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.PropertyRegionalOverrides, key.(string))
}

func findBaseModel(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.BaseModels, key.(string))
}

func findImportedModel(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.ImportedModelMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findModelArtifact(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(string)
	for _, value := range dataset.ModelArtifactMap {
		if item := collections.FindByName(value, k); item != nil {
			return item
		}
	}
	return nil
}

func findEnvironment(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.Environments, key.(string))
}

func findServiceTenancy(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ServiceTenancies, key.(string))
}

func findGpuPool(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.GpuPools, key.(string))
}

func findGpuNode(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.GpuNodeMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findDedicatedAICluster(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.DedicatedAIClusterMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func deleteItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) {
	if key == nil {
		return
	}

	switch category {
	case domain.DedicatedAICluster:
		deleteItemInMap(dataset.DedicatedAIClusterMap, key)
	case domain.GpuNode:
		deleteItemInMap(dataset.GpuNodeMap, key)
	default:
		// exhausive
	}
}

func deleteItemInMap[T models.NamedItem](m map[string][]T, key models.ItemKey) {
	k := key.(models.ScopedItemKey)
	if items, ok := m[k.Scope]; ok {
		items = slices.DeleteFunc(items, func(item T) bool {
			return item.GetName() == k.Name
		})
		m[k.Scope] = items
	}
}

/*
getItemKeyString returns a string representation of the ItemKey.
*/
func getItemKeyString(key models.ItemKey) string {
	if k, ok := key.(string); ok {
		return k
	} else if k, ok := key.(models.ScopedItemKey); ok {
		return fmt.Sprintf("%s/%s", k.Scope, k.Name)
	}

	return "UNKNOWN"
}
