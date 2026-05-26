package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// tuiRowsFlatWith is the shared core of tuiRowsFlat and
// tuiRowsFlatForExport. The cell argument decides whether each cell
// uses the display-mode Render or the export-mode RenderForExport (with
// fallback). The filter + faulty pipeline is identical for both.
func tuiRowsFlatWith[T models.NamedFilterable](s columns.Set[T], items []T, filter string, faultyOnly bool, cell func(columns.Column[T], T) string) []table.Row {
	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
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
// TUI's filter + faulty gates. Display-mode: every cell uses the
// column's Render closure.
func tuiRowsFlat[T models.NamedFilterable](s columns.Set[T], items []T, filter string, faultyOnly bool) []table.Row {
	return tuiRowsFlatWith(s, items, filter, faultyOnly, func(c columns.Column[T], m T) string {
		return c.Render(m)
	})
}

// tuiRowsFlatForExport mirrors tuiRowsFlat but consults each column's
// RenderForExport when present. Used by the CSV export path so
// OCID-shaped columns emit fully-qualified IDs rather than raw
// suffixes.
func tuiRowsFlatForExport[T models.NamedFilterable](s columns.Set[T], items []T, realm, region, filter string, faultyOnly bool) []table.Row {
	return tuiRowsFlatWith(s, items, filter, faultyOnly, func(c columns.Column[T], m T) string {
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
	scopeCategory domain.Category,
	ctx *domain.Scope,
	filter string,
	faultyOnly bool,
	cell func(columns.GroupedColumn[T], string, T) string,
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
				row[j] = cell(c, k, it)
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// tuiRowsGrouped renders a grouped map, applying the scope-context
// gate (key vs name) before filter/faulty. Display-mode: every cell
// uses the column's Render closure with the (group key, item) pair.
func tuiRowsGrouped[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.Scope,
	filter string,
	faultyOnly bool,
) []table.Row {
	return tuiRowsGroupedWith(g, m, scopeCategory, ctx, filter, faultyOnly, func(c columns.GroupedColumn[T], k string, it T) string {
		return c.Render(k, it)
	})
}

// tuiRowsGroupedForExport mirrors tuiRowsGrouped but consults each
// column's RenderForExport when present. Used by the CSV export path so
// OCID-shaped columns emit fully-qualified IDs.
func tuiRowsGroupedForExport[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.Scope,
	realm, region, filter string,
	faultyOnly bool,
) []table.Row {
	return tuiRowsGroupedWith(g, m, scopeCategory, ctx, filter, faultyOnly, func(c columns.GroupedColumn[T], k string, it T) string {
		if c.RenderForExport != nil {
			return c.RenderForExport(realm, region, k, it)
		}
		return c.Render(k, it)
	})
}
