package tui

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		{Title: "Name"}, {Title: "Tenant"}, {Title: "Internal"}, {Title: "Usage"},
		{Title: "Type"}, {Title: "Model"}, {Title: "Shape/Profile"},
		{Title: "Size"}, {Title: "Age"}, {Title: "Status"},
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
