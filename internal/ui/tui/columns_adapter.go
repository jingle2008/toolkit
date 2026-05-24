package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// tuiRowsFlat renders a slice through a flat Set, applying the
// TUI's filter + faulty gates. Mirrors the existing filterRows
// helper but uses the canonical Render closures.
func tuiRowsFlat[T models.NamedFilterable](s columns.Set[T], items []T, filter string, faultyOnly bool) []table.Row {
	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
	}
	matches := collections.FilterSlice(items, nil, filter, pred)
	rows := make([]table.Row, len(matches))
	for i, m := range matches {
		row := make(table.Row, len(s.Columns))
		for j, c := range s.Columns {
			row[j] = c.Render(m)
		}
		rows[i] = row
	}
	return rows
}

// tuiRowsGrouped renders a grouped map, preserving the scope logic
// from filterRowsScoped.
func tuiRowsGrouped[T models.NamedFilterable](
	g columns.GroupedSet[T],
	m map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
	faultyOnly bool,
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
				row[j] = c.Render(k, it)
			}
			rows = append(rows, row)
		}
	}
	return rows
}
