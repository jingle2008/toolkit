package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
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
	for _, cat := range domain.Categories() {
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

// TestUpdateColumns_SortIndicator locks in the header glyph rendering:
// only the active sort column carries an arrow (↑, or ↓ on second press);
// every other column, sortable or not, carries none. (The sort keys'
// underlines are stripped here; TestPrefix_SortUnderlinesHeaderKeys
// covers them.)
func TestUpdateColumns_SortIndicator(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	m.table.SetWidth(200) // wide enough that nothing truncates
	m.category = domain.DedicatedAICluster
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.sortColumn = common.NameCol
	m.sortAsc = true
	m.updateColumns()

	titles := map[string]string{}
	for _, c := range m.table.Columns() {
		title := ansi.Strip(c.Title)
		base, _, _ := strings.Cut(title, " ")
		titles[base] = title
	}
	assert.Equal(t, "Name ↑", titles["Name"])
	assert.Equal(t, "Tenant", titles["Tenant"], "sortable but not active: no glyph")
	assert.Equal(t, "Status", titles["Status"])

	m.sortAsc = false
	m.updateColumns()
	assert.Equal(t, "Name ↓", ansi.Strip(m.table.Columns()[0].Title))
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
		WithLogger(logging.Discard()),
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
	assert.Equal(t, "BM.…00.8", rows[0][6], "every long value is cut in the middle, keeping its tail")
	assert.Equal(t, "ACTIVE", rows[0][9], "Status must not be touched")
}

// A narrow GPUNode table keeps pools that differ only at the end apart:
// cut at the end, both read "gais_inference_h…".
func TestApplyMiddleTruncation_KeepsDistinguishingTail(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUNode
	m.headers = headersFor(m.category)
	pool := slices.IndexFunc(m.headers, func(h header) bool { return h.text == "Pool" })
	require.GreaterOrEqual(t, pool, 0)
	cols := make([]table.Column, len(m.headers))
	for i, h := range m.headers {
		cols[i] = table.Column{Title: h.text, Width: 18}
	}
	m.table.SetColumns(cols)
	row := func(p string) table.Row {
		r := make(table.Row, len(m.headers))
		r[pool] = p
		return r
	}
	rows := []table.Row{row("gais_inference_h100_cn_01"), row("gais_inference_h100_cn_02")}
	m.applyMiddleTruncation(rows)
	assert.True(t, strings.HasSuffix(rows[0][pool], "cn_01"), rows[0][pool])
	assert.True(t, strings.HasSuffix(rows[1][pool], "cn_02"), rows[1][pool])
}

// TestApplyRows_PreservesRawRowForKeyLookup is a regression guard for
// the bug where detail view for DAC/ImportedModel rendered "null":
// applyMiddleTruncation rewrote Name/Tenant cells in place, and the
// downstream itemKeyFrom(category, m.table.SelectedRow()) then built
// a ScopedItemKey from truncated strings — ImportedModelMap lookup
// missed, findItem returned nil, jsonutil.Pretty rendered "null".
// applyRows must preserve a parallel un-truncated copy so the
// selectedRawRow helper can recover the real key.
func TestApplyRows_PreservesRawRowForKeyLookup(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	m.category = domain.DedicatedAICluster
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.headers = headersFor(m.category)
	cols := make([]table.Column, len(m.headers))
	for i, h := range m.headers {
		w := 8
		if h.text == "Status" {
			w = 10
		}
		cols[i] = table.Column{Title: h.text, Width: w}
	}
	m.table.SetColumns(cols)

	rawName := "amaaaaaasxj5imyasw65kzgst7qhopkqbh4hiahgcdpx7gfxesuj7mndycca"
	rawTenant := "amaaaaaatenancysuffixxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	rows := []table.Row{{
		rawName, rawTenant, "true", "50%", "LARGE",
		"llama3", "BM.GPU.H100.8", "4", "2d", "ACTIVE",
	}}
	m.applyRows(rows, nil, false)

	assert.Contains(t, m.table.SelectedRow()[0], "…", "sanity: live row should be truncated")
	raw := m.selectedRawRow()
	assert.Equal(t, rawName, raw[0], "raw row 0 (Name) must keep the original OCID")
	assert.Equal(t, rawTenant, raw[1], "raw row 1 (Tenant) must keep the original OCID")

	// End-to-end: the truncated row would have produced a ScopedItemKey
	// whose Scope misses DedicatedAIClusterMap. Round-trip via
	// selectedRawRow → itemKeyFrom → findItem must resolve to the
	// original DAC pointer, not nil.
	m.dataset = &models.Dataset{
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
			rawTenant: {{Name: rawName, Status: "ACTIVE"}},
		},
	}
	resolved := findItem(m.dataset, m.category, itemKeyFrom(m.category, m.selectedRawRow()))
	require.NotNil(t, resolved, "findItem should resolve the DAC from the raw row")
	dac, ok := resolved.(*models.DedicatedAICluster)
	require.True(t, ok, "resolved item must be *DedicatedAICluster, got %T", resolved)
	assert.Equal(t, rawName, dac.Name)

	// And the truncated row that bubbles serves to the user still misses
	// — proves the bug exists in the absence of selectedRawRow.
	truncatedKey := itemKeyFrom(m.category, m.table.SelectedRow())
	assert.Nil(t, findItem(m.dataset, m.category, truncatedKey),
		"sanity: lookup via the truncated row must miss; if this fires, applyMiddleTruncation isn't truncating")
}
