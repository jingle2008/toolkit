package utils

import (
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestGetBaseModel_AllFields(t *testing.T) {
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
	obj := cty.ObjectVal(map[string]cty.Value{})
	chartValues := map[string]*models.ChartValues{}
	bm := getBaseModel(obj, map[string]struct{}{}, chartValues)
	assert.NotNil(t, bm)
	assert.Empty(t, bm.Capabilities)
}

func TestGetCapability_AllFields(t *testing.T) {
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
	cap := getCapability(obj, chartValues)
	assert.Equal(t, "gen", cap.Capability)
	assert.Equal(t, "cr", cap.CrName)
	assert.Equal(t, "desc", cap.Description)
	assert.Equal(t, "rt", cap.Runtime)
	assert.NotNil(t, cap.ValuesFile)
	assert.Equal(t, "bar.yaml", filepath.Base(*cap.ValuesFile))
	assert.NotNil(t, cap.ChartValues)
}

func TestGetCapability_Empty(t *testing.T) {
	obj := cty.ObjectVal(map[string]cty.Value{})
	chartValues := map[string]*models.ChartValues{}
	cap := getCapability(obj, chartValues)
	assert.NotNil(t, cap)
}
