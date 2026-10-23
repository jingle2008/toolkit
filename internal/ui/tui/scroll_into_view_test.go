package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// TestApplyRows_AutoSelect_ScrollsTargetFullyIntoView guards against an
// off-by-one scroll bug. When returning to a parent category, applyRows
// auto-selects the remembered row (the one whose subcategory we just left).
// For a target beyond the first page the selected row must be fully visible
// in the rendered table, not sitting one line below the bottom edge.
//
// (See also TestApplyRows_LandsOnShortenedName.)
func TestApplyRows_AutoSelect_ScrollsTargetFullyIntoView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	// One narrow column and a small viewport so most rows are off-screen.
	// Clear the stale rows newTestModel left behind before resizing so the
	// SetHeight re-render doesn't index the new single column past its width.
	table.WithColumns([]table.Column{{Title: "Name", Width: 20}})(m.table)
	table.WithRows(nil)(m.table)
	m.table.SetWidth(20) // v2 viewports render nothing at width 0
	m.table.SetHeight(7) // header (2 lines) + ~5 visible rows

	// 30 rows: row-00 .. row-29. Zero-padded so no name is a substring of
	// another (avoids row-2 matching row-20 in the View() contains check).
	// applyRows fits the category's columns to the rows, so each row has
	// a cell per Tenant column.
	rows := make([]table.Row, 30)
	for i := range rows {
		rows[i] = make(table.Row, len(headersFor(domain.Tenant)))
		rows[i][0] = fmt.Sprintf("row-%02d", i)
	}

	// Simulate returning to the parent category after entering a row's
	// subcategory: the scope names the remembered row. Cover a first-page
	// target, one beyond the first page (the off-by-one case), and the last.
	for _, tc := range []struct {
		name string
		want int
	}{
		{"row-02", 2},  // first page
		{"row-20", 20}, // beyond first page — the off-by-one case
		{"row-29", 29}, // last row
	} {
		m.category = domain.Tenant
		m.scope = &domain.Scope{Category: domain.Tenant, Name: tc.name}

		// Simulate real navigation: a category switch first blanks the table
		// via applyRows(nil, ..., false), clearing rawRows so that the
		// subsequent autoSelect call has no prior selection and falls through
		// to findContextIndex (scope). Without this blank step the cursor from
		// the previous iteration would be mistaken for a preserved selection.
		m.applyRows(nil, nil, false)
		// Fresh rows each time, as a reload computes them: applyRows
		// decorates the rows it is given in place.
		m.applyRows(cloneRows(rows), nil, true)

		require.Equal(t, tc.want, m.table.Cursor(),
			"cursor should land on the remembered row %q", tc.name)
		require.True(t, strings.Contains(m.table.View(), tc.name),
			"remembered row %q must be scrolled fully into view, not off by one;\nview:\n%s",
			tc.name, m.table.View())
	}
}

// The cursor lands on the remembered row even when the table is too
// narrow to show its whole name: the name is matched against the plain
// rows, not the shortened ones on screen.
func TestApplyRows_LandsOnShortenedName(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	table.WithRows(nil)(m.table)
	m.table.SetWidth(20)
	m.table.SetHeight(7)
	rows := make([]table.Row, 3)
	for i := range rows {
		rows[i] = make(table.Row, len(headersFor(domain.Tenant)))
		rows[i][0] = fmt.Sprintf("a-very-long-tenant-name-%d", i)
	}
	m.category = domain.Tenant
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "a-very-long-tenant-name-2"}
	m.applyRows(nil, nil, false)
	m.applyRows(cloneRows(rows), nil, true)
	require.Equal(t, 2, m.table.Cursor())
	require.Contains(t, m.table.Rows()[2][0], "…", "precondition: the name is shortened on screen")
}
