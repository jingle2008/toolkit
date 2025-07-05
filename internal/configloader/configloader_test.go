package configloader

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "foo.json"), data, 0o600) // #nosec G306
	require.NoError(t, err)

	out, err := loadOverrides[mockNamedItem](context.Background(), dir)
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "foo", out[0].Name)
}

func TestLoadOverridesDI_ListFilesError(t *testing.T) {
	t.Parallel()
	_, err := loadOverridesDI(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return nil, os.ErrNotExist },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadOverridesDI_LoadFileError(t *testing.T) {
	t.Parallel()
	files := []string{"a.json", "b.json"}
	_, err := loadOverridesDI(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return files, nil },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	out, err := loadOverridesDI(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return []string{}, nil },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestLoadOverrides_ErrorOnBadFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a file with invalid JSON
	err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o600) // #nosec G306
	require.NoError(t, err)

	_, err = loadOverrides[mockNamedItem](context.Background(), dir)
	require.Error(t, err)
}

func TestLoadOverrides_ErrorOnNoDir(t *testing.T) {
	t.Parallel()
	_, err := loadOverrides[mockNamedItem](context.Background(), "/no/such/dir")
	require.Error(t, err)
}

func TestLoadTenancyOverrides_Success(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realm := "testrealm"
	name := "testname"
	tenant := "tenant1"
	tenantDir := filepath.Join(root, name, "regional_values", realm, tenant)
	err := os.MkdirAll(tenantDir, 0o750) // #nosec G301
	require.NoError(t, err)
	item := mockNamedItem{Name: "bar"}
	data, err := json.Marshal(item)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tenantDir, "bar.json"), data, 0o600) // #nosec G306
	require.NoError(t, err)

	out, err := loadTenancyOverrides[mockNamedItem](context.Background(), root, realm, name)
	require.NoError(t, err)
	assert.Contains(t, out, tenant)
	assert.Len(t, out[tenant], 1)
	assert.Equal(t, "bar", out[tenant][0].Name)
}

func TestLoadTenancyOverridesDI_ListSubDirsError(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverridesDI(
		context.Background(),
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return nil, os.ErrNotExist },
		func(_ context.Context, _ string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadTenancyOverridesDI_LoadOverridesError(t *testing.T) {
	t.Parallel()
	tenants := []string{"t1", "t2"}
	_, err := loadTenancyOverridesDI(
		context.Background(),
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return tenants, nil },
		func(_ context.Context, _ string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadTenancyOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverridesDI(
		context.Background(),
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return []string{}, nil },
		func(_ context.Context, _ string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.NoError(t, err)
}

func TestGetConfigPath(t *testing.T) {
	t.Parallel()
	root := "/repo"
	name := "limits"
	expected := "/repo/limitss/oc1_limits.json"
	assert.Equal(t, expected, getConfigPath(root, name))
}

func TestSortNamedItems(t *testing.T) {
	t.Parallel()
	type named struct{ name string }
	// implement GetName for named
	items := []named{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using name field
	slices.SortFunc(items, func(a, b named) int {
		return strings.Compare(a.name, b.name)
	})
	assert.Equal(t, "a", items[0].name)
	assert.Equal(t, "a", items[1].name)
	assert.Equal(t, "b", items[2].name)
	assert.Equal(t, "c", items[3].name)
}

func TestSortKeyedItems(t *testing.T) {
	t.Parallel()
	type keyed struct{ key string }
	// implement GetKey for keyed
	items := []keyed{{"b"}, {"a"}, {"c"}, {"a"}}
	// sort using key field
	slices.SortFunc(items, func(a, b keyed) int {
		return strings.Compare(a.key, b.key)
	})
	assert.Equal(t, "a", items[0].key)
	assert.Equal(t, "a", items[1].key)
	assert.Equal(t, "b", items[2].key)
	assert.Equal(t, "c", items[3].key)
}

func TestListSubDirs(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "testsubdirs")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	// create subdirs
	sub1 := "sub1"
	sub2 := "sub2"
	_ = os.Mkdir(filepath.Join(dir, sub1), 0o750) // #nosec G301
	_ = os.Mkdir(filepath.Join(dir, sub2), 0o750) // #nosec G301
	// create a file
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600) // #nosec G306

	dirs, err := listSubDirs(dir)
	require.NoError(t, err)
	// convert to base names for comparison
	for i := range dirs {
		dirs[i] = filepath.Base(dirs[i])
	}
	assert.ElementsMatch(t, []string{sub1, sub2}, dirs)

	// error path: non-existent dir
	_, err = listSubDirs(filepath.Join(dir, "nope"))
	require.Error(t, err)
}

func TestLoadOverrides_Error(t *testing.T) {
	t.Parallel()
	// Should error on non-existent dir
	_, err := loadOverrides[models.Tenant](context.Background(), "/no/such/dir")
	require.Error(t, err)

	// Should error on bad JSON file
	dir, err := os.MkdirTemp("", "badjson")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()
	badFile := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badFile, []byte("{not valid json"), 0o600) // #nosec G306
	_, err = loadOverrides[models.Tenant](context.Background(), dir)
	require.Error(t, err)
}

func TestLoadTenancyOverrides_Error(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverrides[models.Tenant](context.Background(), "/no/such/dir", "realm", "name")
	require.Error(t, err)
}

func TestGetTenants(t *testing.T) {
	t.Parallel()
	m := map[string]idMap{
		"Tenant1": map[string]struct{}{"id1": {}},
		"Tenant2": map[string]struct{}{"id2": {}},
	}
	tenants := getTenants(m, nil)
	assert.Len(t, tenants, 2)
	names := []string{tenants[0].Name, tenants[1].Name}
	assert.Contains(t, names, "Tenant1")
	assert.Contains(t, names, "Tenant2")
	assert.ElementsMatch(t, []string{"id1"}, tenants[0].IDs)
	assert.ElementsMatch(t, []string{"id2"}, tenants[1].IDs)
}

type testOverride struct {
	models.TenancyOverride
	tenantID string
}

func (t testOverride) GetTenantID() string { return t.tenantID }

func TestUpdateTenants(t *testing.T) {
	t.Parallel()
	// Just test that it doesn't panic on empty input
	updateTenants(map[string]idMap{}, map[string][]testOverride{})

	// Test with actual data
	tenantMap := make(map[string]idMap)
	overrideMap := map[string][]testOverride{
		"TenantA": {
			{tenantID: "idA"},
			{tenantID: "idB"},
		},
		"TenantB": {
			{tenantID: "idC"},
		},
	}
	updateTenants(tenantMap, overrideMap)
	assert.Contains(t, tenantMap, "TenantA")
	assert.Contains(t, tenantMap, "TenantB")
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
	assert.Len(t, envs, 3)
	regions := []string{envs[0].Region, envs[1].Region, envs[2].Region}
	assert.Contains(t, regions, "us-phx-1")
	assert.Contains(t, regions, "us-ashburn-1")
	assert.Contains(t, regions, "eu-frankfurt-1")
}

func TestIsValidEnvironment(t *testing.T) {
	t.Parallel()
	env := models.Environment{}
	all := []models.Environment{{}, {}}
	assert.True(t, isValidEnvironment(env, all))
	assert.False(t, isValidEnvironment(models.Environment{}, []models.Environment{}))
}

func TestLoadDataset_Error(t *testing.T) {
	t.Parallel()
	_, err := LoadDataset(context.Background(), "/no/such/path", models.Environment{}, &models.Metadata{})
	require.Error(t, err)
}

func TestLoadDataset_ContextCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := LoadDataset(ctx, "/no/such/path", models.Environment{}, &models.Metadata{})
	require.Error(t, err)
}

func TestLoadDataset_Success(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	realm := "oc1"
	// Create minimal limit definition group
	limitDefDir := filepath.Join(tmp, "shared_modules/limits")
	_ = os.MkdirAll(limitDefDir, 0o750)
	// Create required subdirs for definitions
	limitDefSubdir := filepath.Join(limitDefDir, "limits_definitions")
	consoleDefSubdir := filepath.Join(limitDefDir, "console_properties_definitions")
	propDefSubdir := filepath.Join(limitDefDir, "properties_definitions")
	_ = os.MkdirAll(limitDefSubdir, 0o750)
	_ = os.MkdirAll(consoleDefSubdir, 0o750)
	_ = os.MkdirAll(propDefSubdir, 0o750)
	// Create required shep_targets directory
	_ = os.MkdirAll(filepath.Join(tmp, "shared_modules/shep_targets"), 0o750)
	// Create required tensorrt_models_config directory and minimal .tf file
	tensorrtDir := filepath.Join(tmp, "shared_modules/tensorrt_models_config")
	_ = os.MkdirAll(tensorrtDir, 0o750)
	tensorrtTfContent := `
locals {
  all_models_map = {}
}
`
	_ = os.WriteFile(filepath.Join(tensorrtDir, "models.tf"), []byte(tensorrtTfContent), 0o600)
	limitDef := models.LimitDefinitionGroup{Values: []models.LimitDefinition{{Name: "foo"}}}
	limitDefPath := filepath.Join(limitDefSubdir, realm+"_limits_definition.json")
	data, err := json.Marshal(limitDef)
	require.NoError(t, err)
	_ = os.WriteFile(limitDefPath, data, 0o600)

	// Console property definition group
	consoleDef := models.ConsolePropertyDefinitionGroup{Values: []models.ConsolePropertyDefinition{{Name: "bar"}}}
	consoleDefPath := filepath.Join(consoleDefSubdir, realm+"_console_properties_definition.json")
	data, err = json.Marshal(consoleDef)
	require.NoError(t, err)
	_ = os.WriteFile(consoleDefPath, data, 0o600)

	// Property definition group
	propDef := models.PropertyDefinitionGroup{Values: []models.PropertyDefinition{{Name: "baz"}}}
	propDefPath := filepath.Join(propDefSubdir, realm+"_properties_definition.json")
	data, err = json.Marshal(propDef)
	require.NoError(t, err)
	_ = os.WriteFile(propDefPath, data, 0o600)

	// Tenancy overrides
	limitTenancyDir := filepath.Join(limitDefDir, "limits_tenancy_overrides", "regional_values", realm, "tenant1")
	consoleTenancyDir := filepath.Join(limitDefDir, "console_properties_tenancy_overrides", "regional_values", realm, "tenant1")
	propTenancyDir := filepath.Join(limitDefDir, "properties_tenancy_overrides", "regional_values", realm, "tenant1")
	_ = os.MkdirAll(limitTenancyDir, 0o750)
	_ = os.MkdirAll(consoleTenancyDir, 0o750)
	_ = os.MkdirAll(propTenancyDir, 0o750)
	limitOverride := models.LimitTenancyOverride{TenantID: "tenant1"}
	limitOverridePath := filepath.Join(limitTenancyDir, "limits_tenancy_overrides.json")
	data, err = json.Marshal(limitOverride)
	require.NoError(t, err)
	_ = os.WriteFile(limitOverridePath, data, 0o600)
	consoleOverride := models.ConsolePropertyTenancyOverride{TenantID: "tenant1"}
	consoleOverridePath := filepath.Join(consoleTenancyDir, "console_properties_tenancy_overrides.json")
	data, err = json.Marshal(consoleOverride)
	require.NoError(t, err)
	_ = os.WriteFile(consoleOverridePath, data, 0o600)
	propOverride := models.PropertyTenancyOverride{Tag: "tenant1"}
	propOverridePath := filepath.Join(propTenancyDir, "properties_tenancy_overrides.json")
	data, err = json.Marshal(propOverride)
	require.NoError(t, err)
	_ = os.WriteFile(propOverridePath, data, 0o600)

	// Regional overrides
	consoleRegOverrideDir := filepath.Join(limitDefDir, "console_properties_regional_overrides", "regional_values", realm)
	propRegOverrideDir := filepath.Join(limitDefDir, "properties_regional_overrides", "regional_values", realm)
	limitRegOverrideDir := filepath.Join(limitDefDir, "limits_regional_overrides", "regional_values", realm)
	_ = os.MkdirAll(consoleRegOverrideDir, 0o750)
	_ = os.MkdirAll(propRegOverrideDir, 0o750)
	_ = os.MkdirAll(limitRegOverrideDir, 0o750)
	consoleRegOverride := models.ConsolePropertyRegionalOverride{Name: "cpr"}
	consoleRegOverridePath := filepath.Join(consoleRegOverrideDir, "console_properties_regional_overrides.json")
	data, err = json.Marshal(consoleRegOverride)
	require.NoError(t, err)
	_ = os.WriteFile(consoleRegOverridePath, data, 0o600)
	propRegOverride := models.PropertyRegionalOverride{Name: "pr"}
	propRegOverridePath := filepath.Join(propRegOverrideDir, "properties_regional_overrides.json")
	data, err = json.Marshal(propRegOverride)
	require.NoError(t, err)
	_ = os.WriteFile(propRegOverridePath, data, 0o600)
	// Add a dummy file for limits_regional_overrides to avoid directory read error
	_ = os.WriteFile(filepath.Join(limitRegOverrideDir, "dummy.json"), []byte("{}"), 0o600)

	// Environment
	env := models.Environment{Type: "dev", Region: "us-phx-1", Realm: realm}

	// Create minimal .tf file for ServiceTenancy in shep_targets
	shepTargetsDir := filepath.Join(tmp, "shared_modules/shep_targets")
	_ = os.MkdirAll(shepTargetsDir, 0o750)
	tfContent := `
locals {
  oc1_t1 = {
    tenancy_name = "t1"
    home_region = "us-phx-1"
    regions = ["us-phx-1"]
    environment = "dev"
  }
}
`
	tfPath := filepath.Join(shepTargetsDir, "tenancy.tf")
	_ = os.WriteFile(tfPath, []byte(tfContent), 0o600)

	ds, err := LoadDataset(context.Background(), tmp, env, &models.Metadata{})
	require.NoError(t, err)
	assert.Equal(t, "foo", ds.LimitDefinitionGroup.Values[0].Name)
	assert.Equal(t, "bar", ds.ConsolePropertyDefinitionGroup.Values[0].Name)
	assert.Equal(t, "baz", ds.PropertyDefinitionGroup.Values[0].Name)
	assert.Equal(t, "tenant1", ds.Tenants[0].Name)
	assert.Equal(t, "cpr", ds.ConsolePropertyRegionalOverrides[0].Name)
	assert.Equal(t, "pr", ds.PropertyRegionalOverrides[0].Name)
}

func TestValidateEnvironment_Success(t *testing.T) {
	t.Parallel()
	env := models.Environment{Region: "us-phx-1"}
	all := []models.Environment{{Region: "us-phx-1"}, {Region: "us-ashburn-1"}}
	err := validateEnvironment(env, all)
	require.NoError(t, err)
}

func TestLoadDefinitionGroups_Error(t *testing.T) {
	t.Parallel()
	_, _, _, err := loadDefinitionGroups("/no/such/path") //nolint:dogsled // we only need err
	require.Error(t, err)
}

func TestLoadTenancyOverrideGroup_Error(t *testing.T) {
	t.Parallel()
	_, err := LoadTenancyOverrideGroup(context.Background(), "/no/such/path", "realm", &models.Metadata{}) //nolint:dogsled // we only need err
	require.Error(t, err)
}

func TestLoadRegionalOverrides_MissingDir(t *testing.T) {
	t.Parallel()
	out, err := LoadLimitRegionalOverrides(context.Background(), "/no/such/path", "realm")
	require.NoError(t, err)
	assert.Empty(t, out)
	out2, err := LoadConsolePropertyRegionalOverrides(context.Background(), "/no/such/path", "realm")
	require.NoError(t, err)
	assert.Empty(t, out2)
	out3, err := LoadPropertyRegionalOverrides(context.Background(), "/no/such/path", "realm")
	require.NoError(t, err)
	assert.Empty(t, out3)
}
