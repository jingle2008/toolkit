package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
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
//
// One-directional by design: this test asserts every Sort* binding
// has a matching column, but NOT that every column has a Sort*
// binding. Many columns (Status, Display Name, Vendor in non-DAC
// categories, etc.) intentionally don't expose a shift+letter
// sort, and adding the reverse assertion would force a binding
// for every column or maintain an opt-out list.
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
			headers := headersFor(cat)
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

	// Suffix-match the glyphs rather than equality so the assertions
	// remain valid if future updateColumns logic ever truncates or
	// otherwise reshapes the leading title text.

	// Name is the active sort (ascending) → " ↑"
	assert.True(t, strings.HasSuffix(titles["Name"], " ↑"),
		"active sort column should end with ↑ glyph, got %q", titles["Name"])
	// Tenant is sortable (SortTenant in DAC catContext) but not active → " ↕"
	assert.True(t, strings.HasSuffix(titles["Tenant"], " ↕"),
		"sortable non-active column should end with ↕ glyph, got %q", titles["Tenant"])
	// Status is NOT sortable in DAC catContext → no glyph suffix
	assert.False(t, strings.HasSuffix(titles["Status"], " ↕"),
		"non-sortable Status should not carry ↕, got %q", titles["Status"])
	assert.False(t, strings.HasSuffix(titles["Status"], " ↑"),
		"non-sortable Status should not carry ↑, got %q", titles["Status"])
	// Model is NOT sortable → no glyph suffix
	assert.False(t, strings.HasSuffix(titles["Model"], " ↕"),
		"non-sortable Model should not carry ↕, got %q", titles["Model"])
}

func TestTruncateMiddle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"abc", 5, "abc"},          // fits, returned unchanged
		{"abc", 3, "abc"},          // exactly fits
		{"abcdefgh", 5, "ab…gh"},   // 5 = 2 head + 1 ellipsis + 2 tail
		{"abcdefgh", 6, "ab…fgh"},  // 6 = 2 head + 1 ellipsis + 3 tail (tail bias)
		{"abcdefgh", 7, "abc…fgh"}, // 7 = 3 head + 1 ellipsis + 3 tail
		{"abcdefgh", 1, "…"},       // width fits exactly one ellipsis cell
		{"abcdefgh", 0, ""},        // zero width → empty (output never exceeds w)
		// Realistic OCID name portion: head reveals the "amaa…" shape, tail reveals the distinguishing characters.
		{"amaaaaaasxj5imyasw65kzgst7qhopkqbh4hiahgcdpx7gfxesuj7mndycca", 12, "amaaa…ndycca"},
	}
	for _, tc := range cases {
		got := truncateMiddle(tc.s, tc.w)
		assert.Equalf(t, tc.want, got, "truncateMiddle(%q, %d)", tc.s, tc.w)
	}
}

// TestApplyMiddleTruncation_DACNameAndTenant proves the integration:
// for a DAC row whose Name and Tenant are long OCID suffixes and
// whose column widths are narrow, applyMiddleTruncation rewrites
// those two cells with a head + ellipsis + tail and leaves the
// other cells alone.
func TestApplyMiddleTruncation_DACNameAndTenant(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.category = domain.DedicatedAICluster
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.headers = headersFor(m.category)
	// Set narrow widths so truncation must occur on Name and Tenant.
	cols := make([]table.Column, len(m.headers))
	for i, h := range m.headers {
		w := 8
		if h.text == "Status" {
			w = 10
		}
		cols[i] = table.Column{Title: h.text, Width: w}
	}
	m.table.SetColumns(cols)

	rows := []table.Row{{
		"amaaaaaasxj5imya...mndycca", // Name col 0 — must middle-truncate
		"amaaaaaatenancysuffix",      // Tenant col 1 — must middle-truncate
		"true",                       // Internal col 2 — short, unchanged
		"50%",                        // Usage col 3 — short, unchanged
		"LARGE",                      // Type col 4 — short, unchanged
		"llama3",                     // Model col 5
		"BM.GPU.H100.8",              // Shape/Profile col 6
		"4",                          // Size col 7
		"2d",                         // Age col 8
		"ACTIVE",                     // Status col 9 — not middle-truncate
	}}
	m.applyMiddleTruncation(rows)

	assert.Contains(t, rows[0][0], "…", "Name should be middle-truncated, got %q", rows[0][0])
	assert.Contains(t, rows[0][1], "…", "Tenant should be middle-truncated, got %q", rows[0][1])
	assert.NotEqual(t, "…", rows[0][0][0:len("…")], "Name should NOT start with ellipsis (middle-truncate keeps head)")
	assert.Equal(t, "true", rows[0][2], "Internal must not be touched")
	assert.Equal(t, "ACTIVE", rows[0][9], "Status must not be touched (not TruncateMiddle)")
}
