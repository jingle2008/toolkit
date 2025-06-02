package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestGetLocalAttributes_Error(t *testing.T) {
	_, err := getLocalAttributes("/no/such/dir")
	assert.Error(t, err)
}

func TestUpdateLocalAttributes_Error(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "notfound.tf")
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(tmp, attrs)
	assert.Error(t, err)
}

func TestMergeObject(t *testing.T) {
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
	type Foo struct{ X int }
	assert.Nil(t, unmarshalYaml[Foo](nil))
}

func TestUnmarshalYaml_Bad(t *testing.T) {
	type Foo struct{ X int }
	bad := "not: [valid"
	assert.Nil(t, unmarshalYaml[Foo](&bad))
}

func TestCreateAvailabilityDomains(t *testing.T) {
	val := createAvailabilityDomains()
	m := val.AsValueMap()
	assert.Contains(t, m, "ad_list")
}

func TestCreateObjectStorageNamespace(t *testing.T) {
	val := createObjectStorageNamespace()
	m := val.AsValueMap()
	assert.Contains(t, m, "objectstorage_namespace")
}

func TestLoadModelCapabilities(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"model1": cty.TupleVal([]cty.Value{cty.StringVal("cap1"), cty.StringVal("cap2")}),
	})
	caps := loadModelCapabilities(obj)
	assert.Contains(t, caps, "model1")
	assert.Contains(t, caps["model1"], "cap1")
	assert.Contains(t, caps["model1"], "cap2")
}

func TestLoadModelReplicas(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{
		"model1": cty.NumberIntVal(3),
	})
	replicas := loadModelReplicas(obj)
	assert.Equal(t, 3, replicas["model1"])
}

func TestConvertChartValues_Nil(t *testing.T) {
	val := convertChartValues(ChartValues{})
	assert.NotNil(t, val)
}

func TestGetServiceTenancy(t *testing.T) {
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
	_, err := loadChartValuesMap("/no/such/dir")
	assert.Error(t, err)
}

// ---- Merged from tf_utils_additional_test.go ----

func TestGetLocalAttributesDI_ListFilesError(t *testing.T) {
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return nil, assert.AnError },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_UpdateLocalAttributesError(t *testing.T) {
	files := []string{"a.tf", "b.tf"}
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { return assert.AnError },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_EmptyFiles(t *testing.T) {
	out, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return []string{}, nil },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out, 0)
}

func TestGetLocalAttributesDI_Success(t *testing.T) {
	files := []string{"a.tf"}
	called := false
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { called = true; return nil },
	)
	assert.NoError(t, err)
	assert.True(t, called)
}
