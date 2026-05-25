package tui

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestExportTableCSV_Success(t *testing.T) {
	t.Parallel()

	// writeCSV uses the canonical column registry (post-normalization
	// with the CLI -o csv path), so the headers and rows come from
	// TenantColumns + dataset.Tenants — not from the live table's
	// pre-rendered rows. Populate the dataset to drive the output.
	headers := []header{
		{text: "Name"},
		{text: "OCIDs"},
		{text: "Internal"},
		{text: "Note"},
	}
	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "Name"},
		{Title: "OCIDs"},
		{Title: "Internal"},
		{Title: "Note"},
	})

	m := &Model{
		headers:     headers,
		table:       &tbl,
		category:    domain.Tenant,
		environment: models.Environment{Region: "us-ashburn-1", Realm: "oc1"},
		dataset: &models.Dataset{
			Tenants: []models.Tenant{
				{Name: "alice", IDs: []string{"ocid1.tenancy.oc1..a"}, IsInternal: true, Note: "n1"},
				{Name: "bob", IDs: []string{"ocid1.tenancy.oc1..b"}, IsInternal: false, Note: "n2"},
			},
		},
		loader: fakeLoader{dataset: &models.Dataset{}},
		logger: fakeLogger{},
	}

	var buf bytes.Buffer
	err := m.writeCSV(&buf)
	require.NoError(t, err)

	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{"Name", "OCIDs", "Internal", "Note"}, records[0])
	assert.Equal(t, []string{"alice", "ocid1.tenancy.oc1..a", "true", "n1"}, records[1])
	assert.Equal(t, []string{"bob", "ocid1.tenancy.oc1..b", "false", "n2"}, records[2])
}

// TestExportTableCSV_DACUsesFullOCIDs locks in the column-registry
// ExportRender behaviour for DAC: Name should expand to the
// realm/region-qualified resource OCID and Tenant should expand to
// the realm-qualified tenancy OCID. Mirrors the substitution the
// CLI -o csv path now also performs.
func TestExportTableCSV_DACUsesFullOCIDs(t *testing.T) {
	t.Parallel()

	headers := []header{
		{text: "Name"},
		{text: "Tenant"},
		{text: "Internal"},
		{text: "Usage"},
		{text: "Type"},
		{text: "Model"},
		{text: "Shape/Profile"},
		{text: "Size"},
		{text: "Age"},
		{text: "Status"},
	}
	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "Name"},
		{Title: "Tenant"},
		{Title: "Internal"},
		{Title: "Usage"},
		{Title: "Type"},
		{Title: "Model"},
		{Title: "Shape/Profile"},
		{Title: "Size"},
		{Title: "Age"},
		{Title: "Status"},
	})

	m := &Model{
		headers:     headers,
		table:       &tbl,
		category:    domain.DedicatedAICluster,
		environment: models.Environment{Region: "me-dubai-1", Realm: "oc1"},
		dataset: &models.Dataset{
			DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
				"aaaaaaaatenant": {{
					Name:     "amaaaaaadac",
					Status:   "ACTIVE",
					TenantID: "aaaaaaaatenant",
				}},
			},
		},
		loader: fakeLoader{dataset: &models.Dataset{}},
		logger: fakeLogger{},
	}

	var buf bytes.Buffer
	err := m.writeCSV(&buf)
	require.NoError(t, err)

	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	// row[0] (Name) is the full DAC OCID; row[1] (Tenant) is the full tenancy OCID.
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-dubai-1.amaaaaaadac", records[1][0])
	assert.Equal(t, "ocid1.tenancy.oc1..aaaaaaaatenant", records[1][1])
}

func TestExportTableCSV_NilModelOrTable(t *testing.T) {
	t.Parallel()
	// Nil model
	var m *Model
	err := m.exportTableCSV("foo.csv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no table data")

	// Nil table
	m2 := &Model{}
	err2 := m2.exportTableCSV("foo.csv")
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "no table data")
}

func TestExportTableCSV_CreateFileError(t *testing.T) {
	t.Parallel()
	// Use a path in a non-existent directory
	m := &Model{
		headers:     []header{{text: "A"}},
		table:       &table.Model{},
		category:    domain.Tenant,
		environment: models.Environment{Region: "us-ashburn-1", Realm: "oc1"},
		dataset:     &models.Dataset{},
		loader:      fakeLoader{dataset: &models.Dataset{}},
		logger:      fakeLogger{},
	}
	badPath := filepath.Join("no_such_dir", "out.csv")
	err := m.exportTableCSV(badPath)
	require.Error(t, err)
}

func TestExportFilename(t *testing.T) {
	t.Parallel()
	m := &Model{
		environment: models.Environment{Region: "iad"},
		category:    domain.Tenant,
		loader:      fakeLoader{dataset: &models.Dataset{}},
		logger:      fakeLogger{},
	}
	got := m.exportFilename()
	assert.Equal(t, "iad-Tenant.csv", got)
}

// TestExportTableCSV_IgnoresInteractiveSort pins the documented
// contract on writeCSV: the live table's interactive sort state is
// NOT preserved on export — rows come out in the dataset's natural
// (filter-and-faulty-applied) order. Before the export-normalization
// commit, writeCSV iterated m.table.Rows() which carried the sort;
// after the rewrite, rows are rebuilt from the dataset via the
// export-mode row builder. Sort drops out. Test guards against an
// accidental revert that would re-couple export to display state.
func TestExportTableCSV_IgnoresInteractiveSort(t *testing.T) {
	t.Parallel()

	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "Name"},
		{Title: "OCIDs"},
		{Title: "Internal"},
		{Title: "Note"},
	})

	m := &Model{
		headers: []header{
			{text: "Name"},
			{text: "OCIDs"},
			{text: "Internal"},
			{text: "Note"},
		},
		table:       &tbl,
		category:    domain.Tenant,
		environment: models.Environment{Region: "us-ashburn-1", Realm: "oc1"},
		dataset: &models.Dataset{
			Tenants: []models.Tenant{
				{Name: "charlie", IDs: []string{"ocid1.tenancy.oc1..c"}},
				{Name: "alice", IDs: []string{"ocid1.tenancy.oc1..a"}},
				{Name: "bob", IDs: []string{"ocid1.tenancy.oc1..b"}},
			},
		},
		// Simulate the user having sorted by Name descending in the
		// live table. Export should ignore this — the rows below
		// must appear in dataset order, not sort order.
		sortColumn: "Name",
		sortAsc:    false,
		loader:     fakeLoader{dataset: &models.Dataset{}},
		logger:     fakeLogger{},
	}

	var buf bytes.Buffer
	require.NoError(t, m.writeCSV(&buf))
	records, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 4) // header + 3 rows
	// Dataset insertion order: charlie, alice, bob. NOT sorted
	// descending by name (would have been: charlie, bob, alice).
	assert.Equal(t, "charlie", records[1][0])
	assert.Equal(t, "alice", records[2][0])
	assert.Equal(t, "bob", records[3][0])
}

// TestExportTUIPathMatchesCLIPath_DAC pins the cross-path invariant
// the CSV snapshot tests cannot directly assert: the TUI export
// (m.exportRows → rowSources[DAC].rows → tuiRowsGroupedForExport)
// must produce the same row content as the CLI export
// (columns.RenderTableForExport) for ExportRender-bearing
// categories. The snapshot tests in internal/cli exercise only the
// CLI path; this guards the TUI path from silently diverging
// (e.g., a future change to the filter pipeline inside
// tuiRowsGroupedWith that doesn't land on the CLI side too).
//
// DAC is the workhorse: both the Name and Tenant columns
// ExportRender into fully-qualified OCIDs, so a per-cell divergence
// surfaces here. Map iteration order differs between the two paths
// (CLI sorts keys; TUI uses Go's randomized iteration), so
// comparison is set-equality, not pointwise.
func TestExportTUIPathMatchesCLIPath_DAC(t *testing.T) {
	t.Parallel()

	realm, region := "oc1", "me-dubai-1"
	fixture := map[string][]models.DedicatedAICluster{
		"aaaaaaaatenant": {
			{Name: "amaaaaaadac1", Status: "ACTIVE", TenantID: "aaaaaaaatenant"},
			{Name: "amaaaaaadac2", Status: "FAILED", TenantID: "aaaaaaaatenant"},
		},
		"bbbbbbbbtenant": {
			{Name: "amaaaaaadac3", Status: "ACTIVE", TenantID: "bbbbbbbbtenant"},
		},
	}

	_, cliRows, err := columns.RenderTableForExport(domain.DedicatedAICluster, fixture, realm, region, nil)
	require.NoError(t, err)

	tuiRows := tuiRowsGroupedForExport(columns.DacColumns, fixture, domain.Tenant, nil, realm, region, "", false)
	got := make([][]string, len(tuiRows))
	for i, r := range tuiRows {
		got[i] = []string(r)
	}

	assert.ElementsMatch(t, cliRows, got)
}

// TestRowSources_CoversAllCategories guards the structural fence
// for rowSources, the single per-category dispatch shared by the
// live table and the CSV export. Adding a new domain.Category
// without a rowSources entry would silently emit empty TUI rows
// AND a header-only CSV on <e>; this test fails the build first.
// Stronger than the previous paired-map parity check, which only
// caught drift between two maps and not "forgot both".
func TestRowSources_CoversAllCategories(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if _, ok := rowSources[cat]; !ok {
			t.Errorf("category %s has no rowSources entry — TUI table and CSV export would both be empty", cat)
		}
	}
}

func TestExportView_ContainsFilenameAndPrompt(t *testing.T) {
	t.Parallel()
	m := &Model{
		environment: models.Environment{Region: "phx"},
		category:    domain.Tenant,
		loader:      fakeLoader{dataset: &models.Dataset{}},
		logger:      fakeLogger{},
	}
	setDefaults(m)
	view := m.exportView()
	assert.Contains(t, view, m.exportFilename())
	assert.Contains(t, view, "Pick an export path:")
}
