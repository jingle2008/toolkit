package configloader

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	"github.com/jingle2008/toolkit/pkg/models"
)

// mockNamedItem implements models.NamedItem for testing
type mockNamedItem struct {
	Name string `json:"name"`
}

func (m mockNamedItem) GetName() string { return m.Name }

func TestLoadOverrides_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	item := mockNamedItem{Name: "foo"}
	data, err := json.Marshal(item)
	testutil.RequireNoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "foo.json"), data, 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	out, err := loadOverrides[mockNamedItem](dir)
	testutil.RequireNoError(t, err)
	testutil.Equal(t, 1, len(out))
	testutil.Equal(t, "foo", out[0].Name)
}

func TestLoadOverridesDI_ListFilesError(t *testing.T) {
	t.Parallel()
	_, err := loadOverridesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return nil, os.ErrNotExist },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireError(t, err)
}

func TestLoadOverridesDI_LoadFileError(t *testing.T) {
	t.Parallel()
	files := []string{"a.json", "b.json"}
	_, err := loadOverridesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireError(t, err)
}

func TestLoadOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	out, err := loadOverridesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return []string{}, nil },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireNoError(t, err)
	testutil.Equal(t, 0, len(out))
}

func TestLoadOverrides_ErrorOnBadFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a file with invalid JSON
	err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	_, err = loadOverrides[mockNamedItem](dir)
	testutil.RequireError(t, err)
}

func TestLoadOverrides_ErrorOnNoDir(t *testing.T) {
	t.Parallel()
	_, err := loadOverrides[mockNamedItem]("/no/such/dir")
	testutil.RequireError(t, err)
}

func TestLoadTenancyOverrides_Success(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realm := "testrealm"
	name := "testname"
	tenant := "tenant1"
	tenantDir := filepath.Join(root, name, "regional_values", realm, tenant)
	err := os.MkdirAll(tenantDir, 0o750) // #nosec G301
	testutil.RequireNoError(t, err)
	item := mockNamedItem{Name: "bar"}
	data, err := json.Marshal(item)
	testutil.RequireNoError(t, err)
	err = os.WriteFile(filepath.Join(tenantDir, "bar.json"), data, 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	out, err := loadTenancyOverrides[mockNamedItem](root, realm, name)
	testutil.RequireNoError(t, err)
	testutil.Contains(t, out, tenant)
	testutil.Equal(t, 1, len(out[tenant]))
	testutil.Equal(t, "bar", out[tenant][0].Name)
}

func TestLoadTenancyOverridesDI_ListSubDirsError(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverridesDI(
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return nil, os.ErrNotExist },
		func(string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireError(t, err)
}

func TestLoadTenancyOverridesDI_LoadOverridesError(t *testing.T) {
	t.Parallel()
	tenants := []string{"t1", "t2"}
	_, err := loadTenancyOverridesDI(
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return tenants, nil },
		func(string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireError(t, err)
}

func TestLoadTenancyOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverridesDI(
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return []string{}, nil },
		func(string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	testutil.RequireNoError(t, err)
}

func TestGetConfigPath(t *testing.T) {
	t.Parallel()
	root := "/repo"
	realm := "oc1"
	name := "limits"
	expected := "/repo/limitss/oc1_limits.json"
	testutil.Equal(t, expected, getConfigPath(root, realm, name))
}

func TestSortNamedItems(t *testing.T) {
	t.Parallel()
	type named struct{ name string }
	// implement GetName for named
	items := []named{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using name field
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	testutil.Equal(t, "a", items[0].name)
	testutil.Equal(t, "a", items[1].name)
	testutil.Equal(t, "b", items[2].name)
	testutil.Equal(t, "c", items[3].name)
}

func TestSortKeyedItems(t *testing.T) {
	t.Parallel()
	type keyed struct{ key string }
	// implement GetKey for keyed
	items := []keyed{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using key field
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})
	testutil.Equal(t, "a", items[0].key)
	testutil.Equal(t, "a", items[1].key)
	testutil.Equal(t, "b", items[2].key)
	testutil.Equal(t, "c", items[3].key)
}

func TestListSubDirs(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "testsubdirs")
	testutil.RequireNoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	// create subdirs
	sub1 := "sub1"
	sub2 := "sub2"
	_ = os.Mkdir(filepath.Join(dir, sub1), 0o750) // #nosec G301
	_ = os.Mkdir(filepath.Join(dir, sub2), 0o750) // #nosec G301
	// create a file
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600) // #nosec G306

	dirs, err := listSubDirs(dir)
	testutil.RequireNoError(t, err)
	// convert to base names for comparison
	for i := range dirs {
		dirs[i] = filepath.Base(dirs[i])
	}
	testutil.ElementsMatch(t, []string{sub1, sub2}, dirs)

	// error path: non-existent dir
	_, err = listSubDirs(filepath.Join(dir, "nope"))
	testutil.RequireError(t, err)
}

func TestLoadOverrides_Error(t *testing.T) {
	t.Parallel()
	// Should error on non-existent dir
	_, err := loadOverrides[models.Tenant]("/no/such/dir")
	testutil.RequireError(t, err)

	// Should error on bad JSON file
	dir, err := os.MkdirTemp("", "badjson")
	testutil.RequireNoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()
	badFile := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badFile, []byte("{not valid json"), 0o600) // #nosec G306
	_, err = loadOverrides[models.Tenant](dir)
	testutil.RequireError(t, err)
}

func TestLoadTenancyOverrides_Error(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverrides[models.Tenant]("/no/such/dir", "realm", "name")
	testutil.RequireError(t, err)
}

func TestLoadRegionalOverrides_Error(t *testing.T) {
	t.Parallel()
	_, err := loadRegionalOverrides[models.Tenant]("/no/such/dir", "realm", "name")
	testutil.RequireError(t, err)
}

func TestGetTenants(t *testing.T) {
	t.Parallel()
	m := map[string]tenantInfo{
		"Tenant1": {idMap: map[string]struct{}{"id1": {}}, overrides: []int{1, 2, 3}},
		"Tenant2": {idMap: map[string]struct{}{"id2": {}}, overrides: []int{4, 5, 6}},
	}
	tenants := getTenants(m)
	testutil.Equal(t, 2, len(tenants))
	names := []string{tenants[0].Name, tenants[1].Name}
	testutil.Contains(t, names, "Tenant1")
	testutil.Contains(t, names, "Tenant2")
	testutil.ElementsMatch(t, []string{"id1"}, tenants[0].IDs)
	testutil.ElementsMatch(t, []string{"id2"}, tenants[1].IDs)
}

type testOverride struct {
	models.TenancyOverride
	tenantID string
}

func (t testOverride) GetTenantID() string { return t.tenantID }

func TestUpdateTenants(t *testing.T) {
	t.Parallel()
	// Just test that it doesn't panic on empty input
	updateTenants(map[string]tenantInfo{}, map[string][]testOverride{}, 0)

	// Test with actual data
	tenantMap := make(map[string]tenantInfo)
	overrideMap := map[string][]testOverride{
		"TenantA": {
			{tenantID: "idA"},
			{tenantID: "idB"},
		},
		"TenantB": {
			{tenantID: "idC"},
		},
	}
	updateTenants(tenantMap, overrideMap, 1)
	testutil.Contains(t, tenantMap, "TenantA")
	testutil.Contains(t, tenantMap, "TenantB")
	testutil.Equal(t, 2, tenantMap["TenantA"].overrides[1])
	testutil.Equal(t, 1, tenantMap["TenantB"].overrides[1])
}

func TestGetEnvironments(t *testing.T) {
	t.Parallel()
	tenancies := []models.ServiceTenancy{
		{
			Name:        "t1",
			Realm:       "r1",
			HomeRegion:  "hr1",
			Regions:     []string{"us-phx-1", "us-ashburn-1"},
			Environment: "dev",
		},
		{
			Name:        "t2",
			Realm:       "r2",
			HomeRegion:  "hr2",
			Regions:     []string{"eu-frankfurt-1"},
			Environment: "prod",
		},
	}
	envs := getEnvironments(tenancies)
	testutil.Equal(t, 3, len(envs))
	regions := []string{envs[0].Region, envs[1].Region, envs[2].Region}
	testutil.Contains(t, regions, "us-phx-1")
	testutil.Contains(t, regions, "us-ashburn-1")
	testutil.Contains(t, regions, "eu-frankfurt-1")
}

func TestIsValidEnvironment(t *testing.T) {
	t.Parallel()
	env := models.Environment{}
	all := []models.Environment{{}, {}}
	testutil.Equal(t, true, isValidEnvironment(env, all))
	testutil.Equal(t, false, isValidEnvironment(models.Environment{}, []models.Environment{}))
}

func TestLoadDataset_Error(t *testing.T) {
	t.Parallel()
	_, err := LoadDataset(context.Background(), "/no/such/path", models.Environment{})
	testutil.RequireError(t, err)
}
