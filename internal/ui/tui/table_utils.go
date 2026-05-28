package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"

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
	domain.GPUPool:            {common.SizeCol, "GPUs"},
	domain.GPUNode:            {"Total", common.FreeCol},
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

// headersFor returns the header strip for a category. Headers are
// precomputed at rowSources construction so the live table, the
// CSV export, and the header strip share one column set. Returns
// nil for unregistered categories (e.g. CategoryUnknown).
func headersFor(category domain.Category) []header {
	if src, ok := rowSources[category]; ok {
		return src.headers
	}
	return nil
}

/*
computeTableRows returns the table rows for a given category, using the appropriate handler.
If the scope is not valid for the category, it is set to nil.
Returns: rows, stats (nil if not applicable)
*/
func computeTableRows(dataset *models.Dataset, category domain.Category, scope *domain.Scope, filter string, sortColumn string, sortAsc bool, faultyOnly bool) ([]table.Row, tableStats) {
	// row-source closures dereference dataset to pull category-specific
	// slices/maps; before the first successful load there's nothing to
	// render. Bail out so refresh-paths driven by user navigation (now
	// reachable since load failures no longer trap the user in
	// ErrorView) don't NPE on Tenant.<Field>.
	if dataset == nil {
		return nil, nil
	}
	if scope != nil && !scope.Category.IsScopeOf(category) {
		scope = nil
	}

	src, exists := rowSources[category]
	if !exists {
		return nil, nil
	}
	rows := src.rows(rowCtx{
		dataset: dataset,
		scope:   scope,
		filter:  filter,
		faulty:  faultyOnly,
	})
	if sortColumn != "" && len(rows) > 0 {
		headers := headersFor(category)
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

	headers := headersFor(category)
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
	headers := headersFor(domain.DedicatedAICluster)
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

/*
itemKeyFrom returns the ItemKey for a given category and table row.
*/
func itemKeyFrom(category domain.Category, row table.Row) models.ItemKey {
	if len(row) == 0 {
		return nil
	}
	switch category {
	case domain.Tenant, domain.LimitDefinition, domain.Environment, domain.ServiceTenancy,
		domain.ConsolePropertyDefinition, domain.PropertyDefinition, domain.GPUPool,
		domain.LimitRegionalOverride, domain.ConsolePropertyRegionalOverride,
		domain.PropertyRegionalOverride, domain.Alias, domain.BaseModel:
		return row[0]
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GPUNode, domain.DedicatedAICluster,
		domain.ImportedModel, domain.ModelArtifact:
		// ModelArtifact row[1] is "Model Internal Name" which equals
		// the ModelArtifactMap's parent BaseModel key — see
		// columns/model_artifact.go. Treating it as a scoped key
		// disambiguates artifacts that share a Name across BaseModels.
		return models.ScopedItemKey{Scope: row[1], Name: row[0]}
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

// cloneRows returns a deep copy of rows so callers can mutate one
// without disturbing the other. Used by applyRows to preserve a
// pre-truncation copy of the table — applyMiddleTruncation mutates
// in place, and itemKeyFrom needs the original Name/Tenant strings.
func cloneRows(rows []table.Row) []table.Row {
	if rows == nil {
		return nil
	}
	out := make([]table.Row, len(rows))
	for i, r := range rows {
		out[i] = append(table.Row(nil), r...)
	}
	return out
}

// selectedRawRow returns the un-truncated row at the table's cursor,
// or the table's (possibly truncated) SelectedRow as a fallback when
// the parallel rawRows index isn't populated. Callers that derive
// ItemKey from cell values (itemKeyFrom) must use this; the live
// SelectedRow may contain "…" in Name/Tenant cells.
func (m *Model) selectedRawRow() table.Row {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rawRows) {
		return m.table.SelectedRow()
	}
	return m.rawRows[idx]
}

// findItem looks up the item identified by (category, key) in the
// dataset. Returns nil for keys that have no matching item, for
// categories that have no rowSource entry, and for categories whose
// rowSource has no find (currently only Alias). Per-category lookup
// logic lives on the rowSource itself — see row_sources.go.
func findItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) any {
	if key == nil {
		return nil
	}
	src, ok := rowSources[category]
	if !ok || src.find == nil {
		return nil
	}
	return src.find(dataset, key)
}

func removeItemFromDataset(dataset *models.Dataset, category domain.Category, key models.ItemKey) {
	if key == nil {
		return
	}

	switch category {
	case domain.DedicatedAICluster:
		deleteItemInMap(dataset.DedicatedAIClusterMap, key)
	case domain.GPUNode:
		deleteItemInMap(dataset.GPUNodeMap, key)
	default:
		// exhaustive
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
itemKeyString returns a string representation of the ItemKey.
*/
func itemKeyString(key models.ItemKey) string {
	if k, ok := key.(string); ok {
		return k
	} else if k, ok := key.(models.ScopedItemKey); ok {
		return fmt.Sprintf("%s/%s", k.Scope, k.Name)
	}

	return "UNKNOWN"
}
