package tui

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

type header struct {
	text  string
	ratio float64
}

type tableStats map[string]int

var statsColumns = map[domain.Category][]string{
	domain.GPUPool:            {common.SizeCol, "GPUs"},
	domain.GPUNode:            {"Total", common.FreeCol},
	domain.GPUWorkload:        {common.GpusCol},
	domain.DedicatedAICluster: {common.SizeCol},
}

// faultyGate reports whether a row, stored under group, is faulty.
type faultyGate func(group string, it models.Faulty) bool

// faultyGateFor returns the Ctrl+Z test for category's rows, nil when
// faultyOnly is off: relations.FaultyRows, the test `toolkit get
// --faulty` and MCP's faulty input apply too.
func faultyGateFor(faultyOnly bool, ds *models.Dataset, category domain.Category) faultyGate {
	if !faultyOnly {
		return nil
	}
	return relations.FaultyRows(ds, category)
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
		dataset:  dataset,
		category: category,
		scope:    scope,
		filter:   filter,
		faulty:   faultyOnly,
	})
	if sortColumn != "" && len(rows) > 0 {
		headers := headersFor(category)
		sortRows(rows, headers, sortColumn, sortAsc)
	}
	return rows, computeStats(category, rows)
}

// rowTotal is how many rows category has in scope before any filter or
// faulty-only mode; with neither, it is shown, so the caller passes n.
func rowTotal(dataset *models.Dataset, category domain.Category, scope *domain.Scope, filter string, faultyOnly bool, n int) int {
	if filter == "" && !faultyOnly {
		return n
	}
	rows, _ := computeTableRows(dataset, category, scope, "", "", true, false)
	return len(rows)
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
		domain.PropertyRegionalOverride, domain.Alias, domain.BaseModel, domain.ServingRuntime:
		return row[0]
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GPUNode, domain.DedicatedAICluster,
		domain.ImportedModel, domain.ModelArtifact, domain.GPUWorkload, domain.InferenceService:
		// ModelArtifact row[1] is "Model Internal Name" which equals
		// the ModelArtifactMap's parent BaseModel key — see
		// columns/model_artifact.go. Treating it as a scoped key
		// disambiguates artifacts that share a Name across BaseModels.
		if len(row) < 2 {
			return nil
		}
		return models.ScopedItemKey{Scope: row[1], Name: row[0]}
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

// cloneRows returns a deep copy of rows so callers can mutate one
// without disturbing the other.
func cloneRows(rows []table.Row) []table.Row {
	if rows == nil {
		return nil
	}
	out := make([]table.Row, len(rows))
	for i, r := range rows {
		out[i] = table.Row(slices.Clone(r))
	}
	return out
}

// selectedRawRow returns the un-truncated row at the table's cursor,
// or the table's (possibly truncated) SelectedRow as a fallback when
// the parallel rawRows index isn't populated. In normal production
// flow rawRows is always populated by applyRows; the fallback exists
// for tests that call m.table.SetRows directly and for genuinely
// empty tables. Callers that derive ItemKey from cell values
// (itemKeyFrom) must use this; the live SelectedRow may contain
// "…" in Name/Tenant cells.
func (m *Model) selectedRawRow() table.Row {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rawRows) {
		return m.table.SelectedRow()
	}
	return m.rawRows[idx]
}

// indexOfItemKey returns the index of the first row whose per-category item
// key equals key, or -1 when key is nil or no row matches. Identity is the
// itemKeyFrom key — the bare Name cell for flat categories, but
// ScopedItemKey{Scope: row[1], Name: row[0]} for scoped ones — so a reload
// re-homes onto the same item even when several visible rows share a Name.
// itemKeyFrom only ever yields a string, a ScopedItemKey, or nil (all
// comparable), so the == comparison is panic-safe.
func indexOfItemKey(rows []table.Row, category domain.Category, key models.ItemKey) int {
	if key == nil {
		return -1
	}
	for i, r := range rows {
		if itemKeyFrom(category, r) == key {
			return i
		}
	}
	return -1
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

// snapshotItem returns a copy of a looked-up item, keeping its pointer
// type, for a tea.Cmd to read off the Update loop: the Cmd then never
// reads the dataset's own element, which Update may replace meanwhile.
// The copy is shallow; Update replaces dataset slices and maps rather
// than writing into them (copy-on-write), so what it shares stays put.
// A missing item is returned as is.
func snapshotItem(item any) any {
	if isNilItem(item) {
		return item
	}
	rv := reflect.ValueOf(item)
	if rv.Kind() != reflect.Pointer {
		return item
	}
	cp := reflect.New(rv.Elem().Type())
	cp.Elem().Set(rv.Elem())
	return cp.Interface()
}

// removeItemFromDataset drops the item from its category's map. The map
// and the group's slice are replaced, not edited, as a tea.Cmd may still
// be reading the old ones (snapshotItem).
func removeItemFromDataset(dataset *models.Dataset, category domain.Category, key models.ItemKey) {
	if key == nil {
		return
	}

	switch category {
	case domain.DedicatedAICluster:
		dataset.DedicatedAIClusterMap = deleteItemInMap(dataset.DedicatedAIClusterMap, key)
	case domain.GPUNode:
		dataset.GPUNodeMap = deleteItemInMap(dataset.GPUNodeMap, key)
	default:
		// exhaustive
	}
}

// deleteItemInMap returns a copy of m without the keyed item; m itself
// is left as it was.
func deleteItemInMap[T models.NamedItem](m map[string][]T, key models.ItemKey) map[string][]T {
	k := key.(models.ScopedItemKey)
	items, ok := m[k.Scope]
	if !ok {
		return m
	}
	out := maps.Clone(m)
	out[k.Scope] = slices.DeleteFunc(slices.Clone(items), func(item T) bool {
		return item.GetName() == k.Name
	})
	return out
}

// updateItemInMap returns a copy of m with fn applied to a copy of the
// keyed item, and whether the item was there; m itself is left as it
// was, for a tea.Cmd still reading it.
func updateItemInMap[T models.NamedItem](m map[string][]T, key models.ScopedItemKey, fn func(*T)) (map[string][]T, bool) {
	items := m[key.Scope]
	i := slices.IndexFunc(items, func(item T) bool { return item.GetName() == key.Name })
	if i < 0 {
		return m, false
	}
	group := slices.Clone(items)
	fn(&group[i])
	out := maps.Clone(m)
	out[key.Scope] = group
	return out, true
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
