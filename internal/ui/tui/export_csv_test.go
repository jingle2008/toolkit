package tui

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportTableCSV_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.csv")

	// Build a minimal Model with headers and table rows
	headers := []header{
		{text: "Name"},
		{text: "Age"},
	}
	rows := []table.Row{
		{"Alice", "30"},
		{"Bob", "25"},
	}
	tbl := table.New()
	tbl.SetColumns([]table.Column{{Title: "Name"}, {Title: "Age"}})
	tbl.SetRows(rows)

	m := &Model{
		headers:     headers,
		table:       &tbl,
		category:    domain.Tenant,
		environment: models.Environment{Region: "us-ashburn-1", Realm: "oc1"},
		dataset:     &models.Dataset{},
		loader:      fakeLoader{dataset: &models.Dataset{}},
		logger:      fakeLogger{},
	}

	err := m.exportTableCSV(outPath)
	require.NoError(t, err)

	// Read and check the CSV file
	// #nosec G304 -- test code, not user input
	f, err := os.Open(outPath)
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{"Name", "Age"}, records[0])
	assert.Equal(t, []string{"Alice", "30"}, records[1])
	assert.Equal(t, []string{"Bob", "25"}, records[2])
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
