package terraform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/jingle2008/toolkit/pkg/models"
)

func poolsConfigDir(t *testing.T, base, sub string) string {
	dir := filepath.Join(base, "shared_modules", sub)
	err := os.MkdirAll(dir, 0o750) // #nosec G301
	require.NoError(t, err)
	return dir
}

func writeTfFile(t *testing.T, dir, name, content string) {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600) // #nosec G306
	require.NoError(t, err)
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
	_, err := GetLocalAttributes(context.Background(), "/no/such/dir")
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractGpuNumber(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
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

func TestLoadGpuPools_PlacementLogicalAdAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}

	ipDir := poolsConfigDir(t, dir, "instance_pools_config")
	// Create required empty dirs for other configs to avoid loader error
	cnDir := poolsConfigDir(t, dir, "cluster_networks_config")
	okeDir := poolsConfigDir(t, dir, "oci_oke_nodepools_config")
	// Write empty locals for the other configs
	emptyLocals := `
locals {
  env_cluster_networks_config = {}
}
`
	err := os.WriteFile(filepath.Join(cnDir, "locals.tf"), []byte(emptyLocals), 0o600)
	require.NoError(t, err)
	emptyLocals2 := `
locals {
  env_nodepools_config = {}
}
`
	err = os.WriteFile(filepath.Join(okeDir, "locals.tf"), []byte(emptyLocals2), 0o600)
	require.NoError(t, err)
	tf := `
locals {
  env_instance_pools_config = {
    "pool1" = {
      shape = "GPU"
      size = 2
      capacity_type = "on-demand"
      placement_logical_ad = "all"
    }
  }
}
`
	err = os.WriteFile(filepath.Join(ipDir, "locals.tf"), []byte(tf), 0o600)
	require.NoError(t, err)

	pools, err := LoadGpuPools(context.Background(), dir, env)
	require.NoError(t, err)
	require.NotNil(t, pools)
	found := false
	for _, p := range pools {
		if p.Name == "pool1" {
			found = true
			assert.Equal(t, "all", p.AvailabilityDomain)
		}
	}
	assert.True(t, found, "pool1 not found in loaded pools")
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
	vals, err := loadLocalValueMap(context.Background(), dir, env)
	require.NoError(t, err)
	assert.NotNil(t, vals)
}

func TestLoadLocalValueMap_NoTfFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	vals, err := loadLocalValueMap(context.Background(), dir, env)
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
	_, err := loadLocalValueMap(context.Background(), dir, env)
	require.Error(t, err)
}

func TestLoadLocalValueMap_CyclicLocals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tf := `
locals {
  a = b
  b = a
}
`
	writeTfFile(t, dir, "cyclic.tf", tf)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	vals, err := loadLocalValueMap(context.Background(), dir, env)
	require.NoError(t, err)
	// Both a and b should not be resolved, so not present in vals
	_, aOk := vals["a"]
	_, bOk := vals["b"]
	assert.False(t, aOk)
	assert.False(t, bOk)
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
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return nil, assert.AnError },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_UpdateLocalAttributesError(t *testing.T) {
	t.Parallel()
	files := []string{"a.tf", "b.tf"}
	_, err := getLocalAttributesDI(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { return assert.AnError },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_EmptyFiles(t *testing.T) {
	t.Parallel()
	out, err := getLocalAttributesDI(
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return []string{}, nil },
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
		context.Background(),
		"irrelevant",
		func(_ context.Context, _, _ string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { called = true; return nil },
	)
	require.NoError(t, err)
	assert.True(t, called)
}
