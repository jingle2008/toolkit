package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestSortBindings_MatchColumnTitles guards the stringly-typed bond
// between keys.Sort* bindings and column titles. The ↕ indicator and
// the sortTableByColumn dispatch both rely on the binding's help
// description (after stripping "Sort ") equaling a real column title
// in the active category. A drift in either direction — a Sort*
// binding pointing at a column that no longer exists, or a renamed
// column whose Sort* binding still uses the old title — would
// silently disable sorting for that column, with no compile-time or
// runtime error.
func TestSortBindings_MatchColumnTitles(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		cat := cat
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()
			km := keys.ResolveKeys(cat, common.ListView)
			sortable := km.SortableColumns()
			if len(sortable) == 0 {
				return // not every category has sort bindings, that's fine
			}
			headers := getHeaders(cat)
			titles := map[string]bool{}
			for _, h := range headers {
				titles[strings.ToLower(h.text)] = true
			}
			for col := range sortable {
				if !titles[col] {
					t.Errorf("category %s: Sort* binding for %q has no matching column title; available: %v",
						cat, col, headerTitles(headers))
				}
			}
		})
	}
}

func headerTitles(hs []header) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.text
	}
	return out
}

// TestUpdateColumns_SortableIndicator locks in the header glyph
// rendering. On the DAC category: the active sort column gets ↑ (or
// ↓ on second press), sortable-but-not-active columns get ↕, and
// non-sortable columns get nothing. Guards against regressions in
// the switch in updateColumns and the SortableColumns helper.
func TestUpdateColumns_SortableIndicator(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.table.SetWidth(200) // wide enough that nothing truncates
	m.category = domain.DedicatedAICluster
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.sortColumn = common.NameCol
	m.sortAsc = true
	m.updateColumns()

	// m.table.Columns() returns the column structs whose Title fields
	// are exactly what updateColumns wrote (no Header style applied
	// at this layer — that happens later, at render time).
	titles := map[string]string{}
	for _, c := range m.table.Columns() {
		base, _, _ := strings.Cut(c.Title, " ")
		titles[base] = c.Title
	}

	// Name is the active sort (ascending) → " ↑"
	assert.Equal(t, "Name ↑", titles["Name"], "active sort column should have ↑ glyph")
	// Tenant is sortable (SortTenant in DAC catContext) but not active → " ↕"
	assert.Equal(t, "Tenant ↕", titles["Tenant"], "sortable non-active column should have ↕ glyph")
	// Status is NOT sortable in DAC catContext → bare title
	assert.Equal(t, "Status", titles["Status"], "non-sortable column should have no glyph")
	// Model is NOT sortable → bare title
	assert.Equal(t, "Model", titles["Model"], "non-sortable column should have no glyph")
}
