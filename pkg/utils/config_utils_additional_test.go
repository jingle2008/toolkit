package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockNamedItem implements models.NamedItem for testing
type mockNamedItem struct {
	Name string `json:"name"`
}

func (m mockNamedItem) GetName() string { return m.Name }

func TestLoadOverrides_Success(t *testing.T) {
	dir := t.TempDir()
	item := mockNamedItem{Name: "foo"}
	data, _ := json.Marshal(item)
	err := os.WriteFile(filepath.Join(dir, "foo.json"), data, 0o644)
	assert.NoError(t, err)

	out, err := loadOverrides[mockNamedItem](dir)
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "foo", out[0].Name)
}

func TestLoadOverridesDI_ListFilesError(t *testing.T) {
	_, err := loadOverridesDI[mockNamedItem](
		"irrelevant",
		func(string, string) ([]string, error) { return nil, assert.AnError },
		func(string) (*mockNamedItem, error) { return nil, nil },
	)
	assert.Error(t, err)
}

func TestLoadOverridesDI_LoadFileError(t *testing.T) {
	files := []string{"a.json", "b.json"}
	_, err := loadOverridesDI[mockNamedItem](
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string) (*mockNamedItem, error) { return nil, assert.AnError },
	)
	assert.Error(t, err)
}

func TestLoadOverridesDI_Empty(t *testing.T) {
	out, err := loadOverridesDI[mockNamedItem](
		"irrelevant",
		func(string, string) ([]string, error) { return []string{}, nil },
		func(string) (*mockNamedItem, error) { return nil, nil },
	)
	assert.NoError(t, err)
	assert.Len(t, out, 0)
}

func TestLoadOverrides_ErrorOnBadFile(t *testing.T) {
	dir := t.TempDir()
	// Write a file with invalid JSON
	err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o644)
	assert.NoError(t, err)

	_, err = loadOverrides[mockNamedItem](dir)
	assert.Error(t, err)
}

func TestLoadOverrides_ErrorOnNoDir(t *testing.T) {
	_, err := loadOverrides[mockNamedItem]("/no/such/dir")
	assert.Error(t, err)
}

func TestLoadTenancyOverrides_Success(t *testing.T) {
	root := t.TempDir()
	realm := "testrealm"
	name := "testname"
	tenant := "tenant1"
	tenantDir := filepath.Join(root, name, "regional_values", realm, tenant)
	err := os.MkdirAll(tenantDir, 0o755)
	assert.NoError(t, err)
	item := mockNamedItem{Name: "bar"}
	data, _ := json.Marshal(item)
	err = os.WriteFile(filepath.Join(tenantDir, "bar.json"), data, 0o644)
	assert.NoError(t, err)

	out, err := loadTenancyOverrides[mockNamedItem](root, realm, name)
	assert.NoError(t, err)
	assert.Contains(t, out, tenant)
	assert.Len(t, out[tenant], 1)
	assert.Equal(t, "bar", out[tenant][0].Name)
}

func TestLoadTenancyOverridesDI_ListSubDirsError(t *testing.T) {
	_, err := loadTenancyOverridesDI[mockNamedItem](
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return nil, assert.AnError },
		func(string) ([]mockNamedItem, error) { return nil, nil },
	)
	assert.Error(t, err)
}

func TestLoadTenancyOverridesDI_LoadOverridesError(t *testing.T) {
	tenants := []string{"t1", "t2"}
	_, err := loadTenancyOverridesDI[mockNamedItem](
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return tenants, nil },
		func(string) ([]mockNamedItem, error) { return nil, assert.AnError },
	)
	assert.Error(t, err)
}

func TestLoadTenancyOverridesDI_Empty(t *testing.T) {
	_, err := loadTenancyOverridesDI[mockNamedItem](
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return []string{}, nil },
		func(string) ([]mockNamedItem, error) { return nil, nil },
	)
	assert.NoError(t, err)
}
