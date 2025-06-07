package terraform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func baseModelsDir(t *testing.T, base string) string {
	dir := filepath.Join(base, "model-serving", "application", "generic_region")
	err := os.MkdirAll(dir, 0o750) // #nosec G301
	require.NoError(t, err)
	return dir
}

func chartValuesDirBM(t *testing.T, base string) string {
	dir := filepath.Join(base, "model-serving", "application", "generic_region", "model_chart_values")
	err := os.MkdirAll(dir, 0o750) // #nosec G301
	require.NoError(t, err)
	return dir
}

func chartValuesDir(t *testing.T, base string) string {
	subdir := filepath.Join(base, "model-serving", "application", "generic_region", "model_chart_values")
	err := os.MkdirAll(subdir, 0o750) // #nosec G301
	require.NoError(t, err)
	return subdir
}

func poolsConfigDir(t *testing.T, base, sub string) string {
	dir := filepath.Join(base, "shared_modules", sub)
	err := os.MkdirAll(dir, 0o750) // #nosec G301
	require.NoError(t, err)
	return dir
}

func writeTfFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600) // #nosec G306
	require.NoError(t, err)
	return path
}

func tensorrtModelsDir(t *testing.T, base string) string {
	dir := filepath.Join(base, "shared_modules", "tensorrt_models_config")
	err := os.MkdirAll(dir, 0o750) // #nosec G301
	require.NoError(t, err)
	return dir
}

func shepTargetsDir(t *testing.T, base string) string {
	subdir := filepath.Join(base, "shared_modules", "shep_targets")
	err := os.MkdirAll(subdir, 0o750) // #nosec G301
	require.NoError(t, err)
	return subdir
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600) // #nosec G306
	require.NoError(t, err)
	return path
}

func TestGetLocalAttributes_Error(t *testing.T) {
	t.Parallel()
	_, err := getLocalAttributes("/no/such/dir")
	assert.Error(t, err)
}

func TestUpdateLocalAttributes_Error(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(os.TempDir(), "notfound.tf")
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(tmp, attrs)
	require.Error(t, err)
}

func TestMergeObject(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{"a": cty.StringVal("x")})
	out := mergeObject(obj, "b", cty.StringVal("y"))
	assert.Equal(t, "x", out.AsValueMap()["a"].AsString())
	assert.Equal(t, "y", out.AsValueMap()["b"].AsString())
}

func TestExtractGpuNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		input    string
		expected int
	}{
		{"with number", "4Gpu", 4},
		{"no number", "Gpu", 0},
		{"empty", "", 0},
		{"non-numeric", "abcGpu", 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractGpuNumber(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestUnmarshalYaml_Nil(t *testing.T) {
	t.Parallel()
	type Foo struct{ X int }
	assert.Nil(t, unmarshalYaml[Foo](nil))
}

func TestUnmarshalYaml_Bad(t *testing.T) {
	t.Parallel()
	type Foo struct{ X int }
	bad := "not: [valid"
	assert.Nil(t, unmarshalYaml[Foo](&bad))
}

func TestCreateAvailabilityDomains(t *testing.T) {
	t.Parallel()
	val := createAvailabilityDomains()
	m := val.AsValueMap()
	assert.Contains(t, m, "ad_list")
}

func TestCreateObjectStorageNamespace(t *testing.T) {
	t.Parallel()
	val := createObjectStorageNamespace()
	m := val.AsValueMap()
	assert.Contains(t, m, "objectstorage_namespace")
}

func TestLoadModelCapabilities(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{
		"model1": cty.TupleVal([]cty.Value{cty.StringVal("cap1"), cty.StringVal("cap2")}),
	})
	caps := loadModelCapabilities(obj)
	assert.Contains(t, caps, "model1")
	assert.Contains(t, caps["model1"], "cap1")
	assert.Contains(t, caps["model1"], "cap2")
}

func TestLoadModelReplicas(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{
		"model1": cty.NumberIntVal(3),
	})
	replicas := loadModelReplicas(obj)
	assert.Equal(t, 3, replicas["model1"])
}

func TestConvertChartValues_Nil(t *testing.T) {
	t.Parallel()
	val := convertChartValues(ChartValues{})
	assert.NotNil(t, val)
}

func TestGetServiceTenancy(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{
		"tenancy_name": cty.StringVal("t1"),
		"home_region":  cty.StringVal("hr"),
		"regions":      cty.TupleVal([]cty.Value{cty.StringVal("r1"), cty.StringVal("r2")}),
		"environment":  cty.StringVal("dev"),
	})
	ten := getServiceTenancy(obj, "realm1")
	assert.Equal(t, "t1", ten.Name)
	assert.Equal(t, "hr", ten.HomeRegion)
	assert.Equal(t, []string{"r1", "r2"}, ten.Regions)
	assert.Equal(t, "dev", ten.Environment)
	assert.Equal(t, "realm1", ten.Realm)
}

func TestLoadChartValuesMap_Error(t *testing.T) {
	t.Parallel()
	_, err := loadChartValuesMap("/no/such/dir")
	assert.Error(t, err)
}

func TestLoadBaseModels_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bmDir := baseModelsDir(t, dir)
	cvDir := chartValuesDirBM(t, dir)

	// Write chart values file
	yaml := `
model:
  name: "test"
`
	err := os.WriteFile(filepath.Join(cvDir, "foo.yaml"), []byte(yaml), 0o600) // #nosec G306
	require.NoError(t, err)

	// Write locals.tf with all required maps
	tf := `
locals {
  enabled_map = {
    "model1" = ["generation"]
  }
  regional_replica_map = {
    "generation" = 1
  }
  base_model_map = {
    "model1" = {
      internal_name = "iname"
      displayName = "dname"
      type = "type"
      category = "cat"
      version = "v1"
      vendor = "ven"
      maxTokens = 42
      vaultKey = "vault"
      isExperimental = true
      isInternal = true
      isLongTermSupported = true
      generation = {
        capability = "gen"
        cr_name = "cr"
        description = "desc"
        runtime = "rt"
        values_file = "foo.yaml"
      }
    }
  }
  deprecation_map = {
    "model1" = {
      baseModelLifeCyclePhase = "active"
      timeDeprecated = "never"
    }
  }
}
`
	err = os.WriteFile(filepath.Join(bmDir, "locals.tf"), []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)

	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	modelsMap, err := LoadBaseModels(context.Background(), dir, env)
	require.NoError(t, err)
	assert.NotNil(t, modelsMap)
	assert.Contains(t, modelsMap, "model1")
	bm := modelsMap["model1"]
	assert.Equal(t, "iname", bm.InternalName)
	assert.Equal(t, "dname", bm.Name)
	assert.Equal(t, "active", bm.LifeCyclePhase)
	assert.Equal(t, "never", bm.TimeDeprecated)
}

func TestLoadBaseModels_MissingLocals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bmDir := baseModelsDir(t, dir)
	tf := `
locals {
  foo = {}
}
`
	err := os.WriteFile(filepath.Join(bmDir, "locals.tf"), []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err = LoadBaseModels(context.Background(), dir, env)
	assert.Error(t, err)
}

func TestLoadChartValuesMap_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chartValuesDir(t, dir)
	// No files in subdir
	out, err := loadChartValuesMap(dir)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestLoadChartValuesMap_ValidYaml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	content := `
model:
  name: "test"
`
	path := filepath.Join(subdir, "foo.yaml")
	err := os.WriteFile(path, []byte(content), 0o600) // #nosec G306
	require.NoError(t, err)

	out, err := loadChartValuesMap(dir)
	require.NoError(t, err)
	assert.Contains(t, out, "foo.yaml")
}

func TestLoadChartValuesMap_SafeReadFileError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	// Create a file and remove read permissions
	path := filepath.Join(subdir, "bad.yaml")
	err := os.WriteFile(path, []byte("foo: bar"), 0o000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(path, 0o600) }() // #nosec G306

	out, err := loadChartValuesMap(dir)
	require.NoError(t, err)
	// Should skip the unreadable file, so out is empty
	assert.Empty(t, out)
}

func TestLoadChartValuesMap_InvalidYaml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	path := filepath.Join(subdir, "bad.yaml")
	err := os.WriteFile(path, []byte("foo: [unclosed"), 0o600) // #nosec G306
	require.NoError(t, err)

	out, err := loadChartValuesMap(dir)
	require.NoError(t, err)
	// Should skip the invalid file, so out is empty
	assert.Empty(t, out)
}

func TestLoadGpuPools_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}

	// instance_pools_config
	ipDir := poolsConfigDir(t, dir, "instance_pools_config")
	tf := `
locals {
  env_instance_pools_config = {
    "pool1" = {
      shape = "GPU"
      size = 2
      capacity_type = "on-demand"
    }
  }
}
`
	err := os.WriteFile(filepath.Join(ipDir, "locals.tf"), []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)

	// cluster_networks_config
	cnDir := poolsConfigDir(t, dir, "cluster_networks_config")
	tf2 := `
locals {
  env_cluster_networks_config = {
    "pool2" = {
      shape = "GPU"
      node_pool_size = 3
      capacity_type = "on-demand"
    }
  }
}
`
	err = os.WriteFile(filepath.Join(cnDir, "locals.tf"), []byte(tf2), 0o600) // #nosec G306
	require.NoError(t, err)

	// oci_oke_nodepools_config
	okeDir := poolsConfigDir(t, dir, "oci_oke_nodepools_config")
	tf3 := `
locals {
  env_nodepools_config = {
    "pool3" = {
      shape = "GPU"
      size = 4
      capacity_type = "on-demand"
    }
  }
}
`
	err = os.WriteFile(filepath.Join(okeDir, "locals.tf"), []byte(tf3), 0o600) // #nosec G306
	require.NoError(t, err)

	pools, err := LoadGpuPools(context.Background(), dir, env)
	require.NoError(t, err)
	assert.NotNil(t, pools)
	assert.GreaterOrEqual(t, len(pools), 3)
}

func TestLoadGpuPools_MissingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	// No config files
	_, err := LoadGpuPools(context.Background(), dir, env)
	require.Error(t, err)
}

func TestLoadLocalValueMap_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
locals {
  foo = "bar"
  num = 42
}
`
	writeTfFile(t, dir, "locals.tf", tf)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	vals, err := loadLocalValueMap(dir, env)
	require.NoError(t, err)
	assert.NotNil(t, vals)
}

func TestLoadLocalValueMap_NoTfFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	vals, err := loadLocalValueMap(dir, env)
	require.NoError(t, err)
	assert.NotNil(t, vals)
	assert.Len(t, vals, 1)
	_, ok := vals["execution_target"]
	assert.True(t, ok)
}

func TestLoadLocalValueMap_InvalidHCL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
locals {
  foo = 
`
	writeTfFile(t, dir, "bad.tf", tf)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err := loadLocalValueMap(dir, env)
	require.Error(t, err)
}

func TestLoadModelArtifacts_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := tensorrtModelsDir(t, dir)
	tf := `
locals {
  all_models_map = {
    "model1" = {
      "trt7" = {
        "A100" = {
          "4Gpu" = "artifact1"
        }
      }
    }
  }
}
`
	err := os.WriteFile(filepath.Join(subdir, "locals.tf"), []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	arts, err := LoadModelArtifacts(context.Background(), dir, env)
	require.NoError(t, err)
	assert.NotNil(t, arts)
	assert.GreaterOrEqual(t, len(arts), 1)
}

func TestLoadModelArtifacts_MissingMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := tensorrtModelsDir(t, dir)
	tf := `
locals {
  foo = {}
}
`
	err := os.WriteFile(filepath.Join(subdir, "locals.tf"), []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err = LoadModelArtifacts(context.Background(), dir, env)
	require.Error(t, err)
}

func TestLoadServiceTenancies_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := shepTargetsDir(t, dir)
	tf := `
locals {
  test_realm_tenancy = {
    tenancy_name = "foo"
    home_region = "us-test-1"
    regions = ["us-test-1"]
    environment = "dev"
  }
}
`
	path := filepath.Join(subdir, "locals.tf")
	err := os.WriteFile(path, []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)

	tenancies, err := LoadServiceTenancies(context.Background(), dir)
	require.NoError(t, err)
	assert.NotNil(t, tenancies)
}

func TestLoadServiceTenancies_GroupAndRegionKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := shepTargetsDir(t, dir)
	tf := `
locals {
  group_foo = {}
  region_groups = {}
}
`
	path := filepath.Join(subdir, "locals.tf")
	err := os.WriteFile(path, []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)

	tenancies, err := LoadServiceTenancies(context.Background(), dir)
	require.NoError(t, err)
	assert.NotNil(t, tenancies)
	assert.Empty(t, tenancies)
}

func TestLoadServiceTenancies_InvalidHCL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := shepTargetsDir(t, dir)
	tf := `
locals {
  foo = 
`
	path := filepath.Join(subdir, "bad.tf")
	err := os.WriteFile(path, []byte(tf), 0o600) // #nosec G306
	require.NoError(t, err)

	_, err = LoadServiceTenancies(context.Background(), dir)
	require.Error(t, err)
}

func TestUpdateLocalAttributes_LocalsBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
locals {
  foo = "bar"
  num = 42
}
`
	path := writeTempFile(t, dir, "locals.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	require.NoError(t, err)
	assert.Contains(t, attrs, "foo")
	assert.Contains(t, attrs, "num")
}

func TestUpdateLocalAttributes_OutputBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
output "baz" {
  value = "qux"
}
`
	path := writeTempFile(t, dir, "output.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	require.NoError(t, err)
	assert.Contains(t, attrs, "baz")
}

func TestUpdateLocalAttributes_NoRelevantBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
resource "null_resource" "test" {}
`
	path := writeTempFile(t, dir, "none.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	require.NoError(t, err)
	assert.Empty(t, attrs)
}

func TestUpdateLocalAttributes_InvalidHCL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
locals {
  foo = 
`
	path := writeTempFile(t, dir, "bad.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	assert.Error(t, err)
}

// ---- Merged from tf_utils_additional_test.go ----

func TestGetLocalAttributesDI_ListFilesError(t *testing.T) {
	t.Parallel()
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return nil, assert.AnError },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_UpdateLocalAttributesError(t *testing.T) {
	t.Parallel()
	files := []string{"a.tf", "b.tf"}
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { return assert.AnError },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_EmptyFiles(t *testing.T) {
	t.Parallel()
	out, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return []string{}, nil },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestGetLocalAttributesDI_Success(t *testing.T) {
	t.Parallel()
	files := []string{"a.tf"}
	called := false
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { called = true; return nil },
	)
	require.NoError(t, err)
	assert.True(t, called)
}

// ---- Merged from tf_utils_get_base_model_test.go ----

func TestGetBaseModel_AllFields(t *testing.T) {
	t.Parallel()
	enabledCaps := map[string]struct{}{
		"generation":    {},
		"summarization": {},
		"chat":          {},
		"embedding":     {},
		"rerank":        {},
	}
	capVal := cty.ObjectVal(map[string]cty.Value{
		"capability":  cty.StringVal("gen"),
		"cr_name":     cty.StringVal("cr"),
		"description": cty.StringVal("desc"),
		"runtime":     cty.StringVal("rt"),
		"values_file": cty.StringVal("foo.yaml"),
	})
	obj := cty.ObjectVal(map[string]cty.Value{
		"internal_name":       cty.StringVal("iname"),
		"displayName":         cty.StringVal("dname"),
		"type":                cty.StringVal("type"),
		"category":            cty.StringVal("cat"),
		"version":             cty.StringVal("v1"),
		"vendor":              cty.StringVal("ven"),
		"maxTokens":           cty.NumberIntVal(42),
		"vaultKey":            cty.StringVal("vault"),
		"isExperimental":      cty.True,
		"isInternal":          cty.True,
		"isLongTermSupported": cty.True,
		"generation":          capVal,
		"summarization":       capVal,
		"chat":                capVal,
		"embedding":           capVal,
		"rerank":              capVal,
	})
	chartValues := map[string]*models.ChartValues{
		"foo.yaml": {Model: &models.ModelSetting{}},
	}
	bm := getBaseModel(obj, enabledCaps, chartValues)
	assert.Equal(t, "iname", bm.InternalName)
	assert.Equal(t, "dname", bm.Name)
	assert.Equal(t, "type", bm.Type)
	assert.Equal(t, "cat", bm.Category)
	assert.Equal(t, "v1", bm.Version)
	assert.Equal(t, "ven", bm.Vendor)
	assert.Equal(t, 42, bm.MaxTokens)
	assert.Equal(t, "vault", bm.VaultKey)
	assert.True(t, bm.IsExperimental)
	assert.True(t, bm.IsInternal)
	assert.True(t, bm.IsLongTermSupported)
	assert.Len(t, bm.Capabilities, 5)
	for _, cap := range bm.Capabilities {
		assert.Equal(t, "gen", cap.Capability)
		assert.Equal(t, "cr", cap.CrName)
		assert.Equal(t, "desc", cap.Description)
		assert.Equal(t, "rt", cap.Runtime)
		assert.NotNil(t, cap.ValuesFile)
		assert.Equal(t, "foo.yaml", *cap.ValuesFile)
		assert.NotNil(t, cap.ChartValues)
	}
}

func TestGetBaseModel_Empty(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{})
	chartValues := map[string]*models.ChartValues{}
	bm := getBaseModel(obj, map[string]struct{}{}, chartValues)
	assert.NotNil(t, bm)
	assert.Empty(t, bm.Capabilities)
}

func TestGetCapability_AllFields(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{
		"capability":  cty.StringVal("gen"),
		"cr_name":     cty.StringVal("cr"),
		"description": cty.StringVal("desc"),
		"runtime":     cty.StringVal("rt"),
		"values_file": cty.StringVal(filepath.Join("foo", "bar.yaml")),
	})
	chartValues := map[string]*models.ChartValues{
		"bar.yaml": {Model: &models.ModelSetting{}},
	}
	capability := getCapability(obj, chartValues)
	assert.Equal(t, "gen", capability.Capability)
	assert.Equal(t, "cr", capability.CrName)
	assert.Equal(t, "desc", capability.Description)
	assert.Equal(t, "rt", capability.Runtime)
	assert.NotNil(t, capability.ValuesFile)
	assert.Equal(t, "bar.yaml", filepath.Base(*capability.ValuesFile))
	assert.NotNil(t, capability.ChartValues)
}

func TestGetCapability_Empty(t *testing.T) {
	t.Parallel()
	obj := cty.ObjectVal(map[string]cty.Value{})
	chartValues := map[string]*models.ChartValues{}
	capability := getCapability(obj, chartValues)
	assert.NotNil(t, capability)
}
