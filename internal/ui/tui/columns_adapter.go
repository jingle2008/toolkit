package tui

import (
	"charm.land/bubbles/v2/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

// tuiRowsFlatWith is the shared core of tuiRowsFlat and
// tuiRowsFlatForExport. The cell argument decides whether each cell
// uses the display-mode Render or the export-mode RenderForExport (with
// fallback). The filter + faulty pipeline is identical for both.
func tuiRowsFlatWith[T models.NamedFilterable](s columns.Set[T], items []T, keep func(T) bool, filter string, faulty faultyGate, cell func(columns.Column[T], T) string) []table.Row {
	pred := keep
	if faulty != nil {
		pred = func(it T) bool { return (keep == nil || keep(it)) && faulty("", it) }
	}
	matches := collections.FilterSlice(items, nil, filter, pred)
	rows := make([]table.Row, len(matches))
	for i, m := range matches {
		row := make(table.Row, len(s.Columns))
		for j, c := range s.Columns {
			row[j] = cell(c, m)
		}
		rows[i] = row
	}
	return rows
}

// tuiRowsFlat renders a slice through a flat Set, applying the
// TUI's filter + faulty gates and, when keep is non-nil, narrowing to
// the items it accepts (the active parent scope). Display-mode: every
// cell uses the column's Render closure.
func tuiRowsFlat[T models.NamedFilterable](s columns.Set[T], items []T, keep func(T) bool, filter string, faulty faultyGate) []table.Row {
	return tuiRowsFlatWith(s, items, keep, filter, faulty, func(c columns.Column[T], m T) string {
		return c.Render(m)
	})
}

// tuiRowsFlatForExport mirrors tuiRowsFlat but consults each column's
// RenderForExport when present. Used by the CSV export path so
// OCID-shaped columns emit fully-qualified IDs rather than raw
// suffixes.
func tuiRowsFlatForExport[T models.NamedFilterable](s columns.Set[T], items []T, keep func(T) bool, realm, region, filter string, faulty faultyGate) []table.Row {
	return tuiRowsFlatWith(s, items, keep, filter, faulty, func(c columns.Column[T], m T) string {
		if c.RenderForExport != nil {
			return c.RenderForExport(realm, region, m)
		}
		return c.Render(m)
	})
}

// tuiRowsGroupedWith is the shared core of tuiRowsGrouped and
// tuiRowsGroupedForExport. The cell argument selects display vs
// export rendering; the scope-context + filter + faulty pipeline is
// identical for both.
func tuiRowsGroupedWith[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	parents relations.Grouped[T],
	ctx *domain.Scope,
	filter string,
	faulty faultyGate,
	cell func(columns.GroupedColumn[T], string, T) string,
) []table.Row {
	if faulty != nil {
		m = keepFaulty(m, faulty)
	}
	matches := parents.Filter(m, ctx, filter, nil)
	rows := make([]table.Row, 0)
	for k, items := range matches {
		for _, it := range items {
			row := make(table.Row, len(g.Columns))
			for j, c := range g.Columns {
				row[j] = cell(c, k, it)
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// tuiRowsGrouped renders a grouped map, applying the parent-scope gate
// before filter/faulty. Display-mode: every cell
// uses the column's Render closure with the (group key, item) pair.
func tuiRowsGrouped[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	parents relations.Grouped[T],
	ctx *domain.Scope,
	filter string,
	faulty faultyGate,
) []table.Row {
	return tuiRowsGroupedWith(g, m, parents, ctx, filter, faulty, func(c columns.GroupedColumn[T], k string, it T) string {
		return c.Render(k, it)
	})
}

// tuiRowsGroupedForExport mirrors tuiRowsGrouped but consults each
// column's RenderForExport when present. Used by the CSV export path so
// OCID-shaped columns emit fully-qualified IDs.
func tuiRowsGroupedForExport[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	parents relations.Grouped[T],
	ctx *domain.Scope,
	realm, region, filter string,
	faulty faultyGate,
) []table.Row {
	return tuiRowsGroupedWith(g, m, parents, ctx, filter, faulty, func(c columns.GroupedColumn[T], k string, it T) string {
		if c.RenderForExport != nil {
			return c.RenderForExport(realm, region, k, it)
		}
		return c.Render(k, it)
	})
}

// keepFaulty returns the rows of m that faulty accepts, by group; a group
// left with none is dropped. The gate needs the group (a row's identity
// may include it), which the scope filter's predicate doesn't get.
func keepFaulty[T models.Faulty](m map[string][]T, faulty faultyGate) map[string][]T {
	out := make(map[string][]T, len(m))
	for g, items := range m {
		var kept []T
		for _, it := range items {
			if faulty(g, it) {
				kept = append(kept, it)
			}
		}
		if len(kept) > 0 {
			out[g] = kept
		}
	}
	return out
}
