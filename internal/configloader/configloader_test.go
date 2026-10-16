package configloader

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	_, err := loadOverridesWith(
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
	_, err := loadOverridesWith(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return files, nil },
		func(string) (*mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	out, err := loadOverridesWith(
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
	_, err := loadTenancyOverridesWith(
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
	_, err := loadTenancyOverridesWith(
		context.Background(),
		"irrelevant", "realm", "name",
		func(string) ([]string, error) { return tenants, nil },
		func(_ context.Context, _ string) ([]mockNamedItem, error) { return nil, os.ErrNotExist },
	)
	require.Error(t, err)
}

func TestLoadTenancyOverridesDI_Empty(t *testing.T) {
	t.Parallel()
	_, err := loadTenancyOverridesWith(
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
	m := map[string]idSet{
		"Tenant2": map[string]struct{}{"id2": {}},
		"Tenant1": map[string]struct{}{"id1b": {}, "id1a": {}},
	}
	tenants, dirToName := getTenants(m, nil)
	require.Len(t, tenants, 2)
	// Sorted by name, IDs sorted — independent of map iteration order.
	assert.Equal(t, "Tenant1", tenants[0].Name)
	assert.Equal(t, []string{"id1a", "id1b"}, tenants[0].IDs)
	assert.Equal(t, "Tenant2", tenants[1].Name)
	assert.Equal(t, []string{"id2"}, tenants[1].IDs)
	assert.Equal(t, map[string]string{"Tenant1": "Tenant1", "Tenant2": "Tenant2"}, dirToName)
}

// TestGetTenants_SharedIDsFlagged pins that two override directories
// carrying the same tenancy OCID (e.g. "OABCS1" and "oabcs1") stay
// separate tenants but are both flagged faulty via SharedWith.
func TestGetTenants_SharedIDsFlagged(t *testing.T) {
	t.Parallel()
	m := map[string]idSet{
		"oabcs1": {"idX": {}},
		"OABCS1": {"idX": {}},
		"other":  {"idY": {}},
	}
	tenants, _ := getTenants(m, nil)
	require.Len(t, tenants, 3)
	assert.Equal(t, "OABCS1", tenants[0].Name)
	assert.Equal(t, []string{"oabcs1"}, tenants[0].SharedWith)
	assert.True(t, tenants[0].IsFaulty())
	assert.Equal(t, "oabcs1", tenants[1].Name)
	assert.Equal(t, []string{"OABCS1"}, tenants[1].SharedWith)
	assert.Equal(t, "other", tenants[2].Name)
	assert.Empty(t, tenants[2].SharedWith)
	assert.False(t, tenants[2].IsFaulty())
}

// TestGetTenants_SameNameMerged pins that directories resolving to the
// same tenant name through metadata collapse into one Tenant, and that
// both directories map to it.
func TestGetTenants_SameNameMerged(t *testing.T) {
	t.Parallel()
	m := map[string]idSet{
		"rbcpreprod":  {"idR": {}},
		"rbc_preprod": {"idR": {}},
	}
	name := "rbc-preprod"
	internal := false
	tenants, dirToName := getTenants(m, []models.TenantMetadata{{ID: "idR", Name: &name, IsInternal: &internal}})
	require.Len(t, tenants, 1)
	assert.Equal(t, name, tenants[0].Name)
	assert.Equal(t, []string{"idR"}, tenants[0].IDs)
	assert.Empty(t, tenants[0].SharedWith)
	assert.False(t, tenants[0].IsInternal)
	assert.Equal(t, map[string]string{"rbcpreprod": name, "rbc_preprod": name}, dirToName)
}

func TestRekeyByTenant(t *testing.T) {
	t.Parallel()
	m := map[string][]string{"b": {"b1"}, "a": {"a1", "a2"}, "keep": {"k"}}
	got := rekeyByTenant(m, map[string]string{"a": "T", "b": "T"})
	assert.Equal(t, map[string][]string{"T": {"a1", "a2", "b1"}, "keep": {"k"}}, got)
}

type testOverride struct {
	models.TenancyOverride
	tenantID string
}

func (t testOverride) GetTenantID() string { return t.tenantID }

func TestUpdateTenants(t *testing.T) {
	t.Parallel()
	// Just test that it doesn't panic on empty input
	updateTenants(map[string]idSet{}, map[string][]testOverride{})

	// Test with actual data
	tenantMap := make(map[string]idSet)
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
	propOverride := models.PropertyTenancyOverride{TenantID: "tenant1"}
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

// TestLoadTenancyOverrideGroup_PopulatesTenantName pins the
// post-load step in LoadTenancyOverrideGroup that copies the tenant
// directory name onto each override's TenantName field. This is the
// invariant that lets MCP and CLI flatten the maps through plain
// output.Flatten (instead of the old FlattenWithKey injection path)
// while preserving the "tenant" JSON field shape.
func TestLoadTenancyOverrideGroup_PopulatesTenantName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realm := "testrealm"
	writeOverride := func(category, tenant, filename, body string) {
		t.Helper()
		dir := filepath.Join(root, "shared_modules/limits", category+"_tenancy_overrides", "regional_values", realm, tenant)
		require.NoError(t, os.MkdirAll(dir, 0o750)) // #nosec G301
		require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o600))
	}
	// Two limit overrides under one tenant, plus one each for console_property
	// and property categories.
	writeOverride("limits", "tenant-a", "lim.json", `{"name":"l1","tenant_id":"ocid1.tenancy.aaa"}`)
	writeOverride("console_properties", "tenant-a", "cp.json", `{"name":"c1","tenant_id":"ocid1.tenancy.aaa"}`)
	writeOverride("properties", "tenant-b", "p.json", `{"name":"p1","tag":"ocid1.tenancy.bbb"}`)

	group, err := LoadTenancyOverrideGroup(context.Background(), root, realm, &models.Metadata{})
	require.NoError(t, err)

	for _, ov := range group.LimitTenancyOverrideMap["tenant-a"] {
		assert.Equal(t, "tenant-a", ov.TenantName, "Limit override TenantName must equal the directory name")
	}
	for _, ov := range group.ConsolePropertyTenancyOverrideMap["tenant-a"] {
		assert.Equal(t, "tenant-a", ov.TenantName, "ConsoleProperty override TenantName must equal the directory name")
	}
	for _, ov := range group.PropertyTenancyOverrideMap["tenant-b"] {
		assert.Equal(t, "tenant-b", ov.TenantName, "Property override TenantName must equal the directory name")
	}
}

// TestLoadTenancyOverrideGroup_EmptyTenantDir verifies that an
// existing tenant directory containing zero override files is
// handled safely: the map entry is present with an empty slice,
// and the TenantName-population pass is a no-op (no panic, no
// stray empty record).
func TestLoadTenancyOverrideGroup_EmptyTenantDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realm := "testrealm"
	// Create an empty tenant directory under each of the three categories.
	for _, category := range []string{"limits", "console_properties", "properties"} {
		dir := filepath.Join(root, "shared_modules/limits", category+"_tenancy_overrides", "regional_values", realm, "tenant-empty")
		require.NoError(t, os.MkdirAll(dir, 0o750)) // #nosec G301
	}

	group, err := LoadTenancyOverrideGroup(context.Background(), root, realm, &models.Metadata{})
	require.NoError(t, err)

	// Empty slices for the empty tenant — map presence + zero-length slice.
	assert.Empty(t, group.LimitTenancyOverrideMap["tenant-empty"])
	assert.Empty(t, group.ConsolePropertyTenancyOverrideMap["tenant-empty"])
	assert.Empty(t, group.PropertyTenancyOverrideMap["tenant-empty"])
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

func TestMerge_PrimitivesAndPointers(t *testing.T) {
	t.Parallel()
	// string
	assert.Equal(t, "a", merge("a", nil))
	b := "b"
	assert.Equal(t, "b", merge("a", &b))
	// bool
	assert.True(t, merge(true, nil))
	bb := false
	assert.False(t, merge(true, &bb))
}

func TestGetTenants_MergeAndUnmatched(t *testing.T) {
	t.Parallel()
	// tenantMap has two tenants, metadata has one matching and one extra
	tenantMap := map[string]idSet{
		"TenantA": {"idA": {}},
		"TenantB": {"idB": {}},
	}
	name := "TenantA-override"
	extra := "TenantC"
	isInternal := false
	note := "note"
	tenantMeta := []models.TenantMetadata{
		{ID: "idA", Name: &name, IsInternal: &isInternal, Note: &note},
		{ID: "idC", Name: &extra, IsInternal: &isInternal, Note: &note},
	}
	tenants, dirToName := getTenants(tenantMap, tenantMeta)
	// TenantA renamed by metadata, TenantB as is, idC as extra; sorted by name.
	require.Len(t, tenants, 3)
	assert.Equal(t, models.Tenant{Name: name, IDs: []string{"idA"}, IsInternal: isInternal, Note: note}, tenants[0])
	assert.Equal(t, models.Tenant{Name: "TenantB", IDs: []string{"idB"}, IsInternal: true}, tenants[1])
	assert.Equal(t, models.Tenant{Name: extra, IDs: []string{"idC"}, IsInternal: isInternal, Note: note}, tenants[2])
	assert.Equal(t, map[string]string{"TenantA": name, "TenantB": "TenantB"}, dirToName)
}

// TestGetTenants_MetadataOnlyNameCollision pins that a metadata-only
// entry whose name matches an existing tenant is merged into it rather
// than producing two tenants with the same name.
func TestGetTenants_MetadataOnlyNameCollision(t *testing.T) {
	t.Parallel()
	name := "TenantA"
	internal := true
	tenants, _ := getTenants(map[string]idSet{"TenantA": {"idA": {}}},
		[]models.TenantMetadata{{ID: "idC", Name: &name, IsInternal: &internal}})
	require.Len(t, tenants, 1)
	assert.Equal(t, []string{"idA", "idC"}, tenants[0].IDs)
	assert.True(t, tenants[0].IsFaulty())
}

// TestLoadTenancyOverrideGroup_RenamedTenantRekeysOverrides pins the fix
// for metadata-renamed tenants: the override maps must be keyed by the
// resolved Tenant.Name (the scope key), while each override's TenantName
// keeps the source directory name.
func TestLoadTenancyOverrideGroup_RenamedTenantRekeysOverrides(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realm := "testrealm"
	for _, category := range []string{"limits", "console_properties", "properties"} {
		dir := filepath.Join(root, "shared_modules/limits", category+"_tenancy_overrides", "regional_values", realm, "tenant-dir")
		require.NoError(t, os.MkdirAll(dir, 0o750)) // #nosec G301
	}
	dir := filepath.Join(root, "shared_modules/limits", "limits_tenancy_overrides", "regional_values", realm, "tenant-dir")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lim.json"),
		[]byte(`{"name":"l1","tenant_id":"ocid1.tenancy.testrealm..aaa"}`), 0o600))

	name := "Friendly Name"
	internal := false
	meta := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.testrealm..aaa", Name: &name, IsInternal: &internal},
	}}
	group, err := LoadTenancyOverrideGroup(context.Background(), root, realm, meta)
	require.NoError(t, err)

	require.Len(t, group.Tenants, 1)
	assert.Equal(t, name, group.Tenants[0].Name)
	require.Len(t, group.LimitTenancyOverrideMap[name], 1)
	assert.NotContains(t, group.LimitTenancyOverrideMap, "tenant-dir")
	assert.Equal(t, "tenant-dir", group.LimitTenancyOverrideMap[name][0].TenantName)
}

// writeDefinitionRepo seeds the three oc1 definition files (and, when
// splat is non-empty, the splat throttle definitions) under a temp repo.
func writeDefinitionRepo(t *testing.T, splat string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, "shared_modules/limits", rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750)) // #nosec G301
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	write("limits_definitions/oc1_limits_definition.json", `{"group":"ai","values":[{"name":"dedicated-ai-cluster-count","scope":"REGION"}]}`)
	write("console_properties_definitions/oc1_console_properties_definition.json", `{"group":"ai","values":[]}`)
	write("properties_definitions/oc1_properties_definition.json", `{"group":"ai","values":[]}`)
	if splat != "" {
		write("limits_definitions/splat_limit_definitions.json", splat)
	}
	return root
}

// TestLoadDefinitionGroups_Splat pins that the splat throttle definitions
// are appended to the limit definitions, as the Terraform module does.
func TestLoadDefinitionGroups_Splat(t *testing.T) {
	t.Parallel()
	root := writeDefinitionRepo(t, `[{"name":"list-semantic-stores-user","operation_id":"ListSemanticStores",
		"description":"Dynamic throttle","throttle_dimension":"user","type":"STRING","default_value":"50RPS400RPM"}]`)
	limits, _, _, err := loadDefinitionGroups(root)
	require.NoError(t, err)
	require.Len(t, limits.Values, 2)
	assert.Equal(t, "dedicated-ai-cluster-count", limits.Values[0].Name)
	assert.Equal(t, models.LimitDefinition{
		Name: "list-semantic-stores-user", Description: "Dynamic throttle", Type: "STRING",
		OperationID: "ListSemanticStores", ThrottleDimension: "user", DefaultValue: "50RPS400RPM",
	}, limits.Values[1])
	assert.Contains(t, limits.Values[1].FilterableFields(), "ListSemanticStores")
}

func TestLoadDefinitionGroups_SplatAbsentOrBroken(t *testing.T) {
	t.Parallel()
	limits, _, _, err := loadDefinitionGroups(writeDefinitionRepo(t, ""))
	require.NoError(t, err, "a repo without the splat file still loads")
	assert.Len(t, limits.Values, 1)

	_, _, _, err = loadDefinitionGroups(writeDefinitionRepo(t, `{not json`)) //nolint:dogsled // only err matters
	require.Error(t, err)
}
