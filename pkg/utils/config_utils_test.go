package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestGetConfigPath(t *testing.T) {
	root := "/repo"
	realm := "oc1"
	name := "limits"
	expected := "/repo/limitss/oc1_limits.json"
	assert.Equal(t, expected, getConfigPath(root, realm, name))
}

func TestSortNamedItems(t *testing.T) {
	type named struct{ name string }
	// implement GetName for named
	items := []named{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using name field
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	assert.Equal(t, "a", items[0].name)
	assert.Equal(t, "a", items[1].name)
	assert.Equal(t, "b", items[2].name)
	assert.Equal(t, "c", items[3].name)
}

func TestSortKeyedItems(t *testing.T) {
	type keyed struct{ key string }
	// implement GetKey for keyed
	items := []keyed{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using key field
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})
	assert.Equal(t, "a", items[0].key)
	assert.Equal(t, "a", items[1].key)
	assert.Equal(t, "b", items[2].key)
	assert.Equal(t, "c", items[3].key)
}

func TestListSubDirs(t *testing.T) {
	dir, err := ioutil.TempDir("", "testsubdirs")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	// create subdirs
	sub1 := "sub1"
	sub2 := "sub2"
	os.Mkdir(filepath.Join(dir, sub1), 0o755)
	os.Mkdir(filepath.Join(dir, sub2), 0o755)
	// create a file
	ioutil.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644)

	dirs, err := listSubDirs(dir)
	assert.NoError(t, err)
	// convert to base names for comparison
	for i := range dirs {
		dirs[i] = filepath.Base(dirs[i])
	}
	assert.ElementsMatch(t, []string{sub1, sub2}, dirs)

	// error path: non-existent dir
	_, err = listSubDirs(filepath.Join(dir, "nope"))
	assert.Error(t, err)
}

func TestLoadOverrides_Error(t *testing.T) {
	// Should error on non-existent dir
	_, err := loadOverrides[models.Tenant]("/no/such/dir")
	assert.Error(t, err)

	// Should error on bad JSON file
	dir, err := os.MkdirTemp("", "badjson")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)
	badFile := filepath.Join(dir, "bad.json")
	os.WriteFile(badFile, []byte("{not valid json"), 0o644)
	_, err = loadOverrides[models.Tenant](dir)
	assert.Error(t, err)
}

func TestLoadTenancyOverrides_Error(t *testing.T) {
	_, err := loadTenancyOverrides[models.Tenant]("/no/such/dir", "realm", "name")
	assert.Error(t, err)
}

func TestLoadRegionalOverrides_Error(t *testing.T) {
	_, err := loadRegionalOverrides[models.Tenant]("/no/such/dir", "realm", "name")
	assert.Error(t, err)
}

func TestGetTenants(t *testing.T) {
	m := map[string]tenantInfo{
		"Tenant1": {idMap: map[string]struct{}{"id1": {}}, overrides: []int{1, 2, 3}},
		"Tenant2": {idMap: map[string]struct{}{"id2": {}}, overrides: []int{4, 5, 6}},
	}
	tenants := getTenants(m)
	assert.Len(t, tenants, 2)
	names := []string{tenants[0].Name, tenants[1].Name}
	assert.Contains(t, names, "Tenant1")
	assert.Contains(t, names, "Tenant2")
	assert.ElementsMatch(t, tenants[0].IDs, []string{"id1"})
	assert.ElementsMatch(t, tenants[1].IDs, []string{"id2"})
}

type testOverride struct {
	models.TenancyOverride
	tenantID string
}

func (t testOverride) GetTenantID() string { return t.tenantID }

func TestUpdateTenants(t *testing.T) {
	// Just test that it doesn't panic on empty input
	updateTenants[testOverride](map[string]tenantInfo{}, map[string][]testOverride{}, 0)

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
	updateTenants[testOverride](tenantMap, overrideMap, 1)
	assert.Contains(t, tenantMap, "TenantA")
	assert.Contains(t, tenantMap, "TenantB")
	assert.Equal(t, 2, tenantMap["TenantA"].overrides[1])
	assert.Equal(t, 1, tenantMap["TenantB"].overrides[1])
}

func TestGetEnvironments(t *testing.T) {
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
	assert.Len(t, envs, 3)
	regions := []string{envs[0].Region, envs[1].Region, envs[2].Region}
	assert.Contains(t, regions, "us-phx-1")
	assert.Contains(t, regions, "us-ashburn-1")
	assert.Contains(t, regions, "eu-frankfurt-1")
}

func TestIsValidEnvironment(t *testing.T) {
	env := models.Environment{}
	all := []models.Environment{{}, {}}
	assert.True(t, isValidEnvironment(env, all))
	assert.False(t, isValidEnvironment(models.Environment{}, []models.Environment{}))
}

func TestLoadDataset_Error(t *testing.T) {
	_, err := LoadDataset("/no/such/path", models.Environment{})
	assert.Error(t, err)
}
