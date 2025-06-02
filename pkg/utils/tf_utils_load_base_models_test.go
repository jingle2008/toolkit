package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func baseModelsDir(t *testing.T, base string) string {
	dir := filepath.Join(base, "model-serving", "application", "generic_region")
	err := os.MkdirAll(dir, 0o755)
	assert.NoError(t, err)
	return dir
}

func chartValuesDirBM(t *testing.T, base string) string {
	dir := filepath.Join(base, "model-serving", "application", "generic_region", "model_chart_values")
	err := os.MkdirAll(dir, 0o755)
	assert.NoError(t, err)
	return dir
}

func TestLoadBaseModels_Success(t *testing.T) {
	dir := t.TempDir()
	bmDir := baseModelsDir(t, dir)
	cvDir := chartValuesDirBM(t, dir)

	// Write chart values file
	yaml := `
model:
  name: "test"
`
	err := os.WriteFile(filepath.Join(cvDir, "foo.yaml"), []byte(yaml), 0o644)
	assert.NoError(t, err)

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
	err = os.WriteFile(filepath.Join(bmDir, "locals.tf"), []byte(tf), 0o644)
	assert.NoError(t, err)

	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	modelsMap, err := LoadBaseModels(dir, env)
	assert.NoError(t, err)
	assert.NotNil(t, modelsMap)
	assert.Contains(t, modelsMap, "model1")
	bm := modelsMap["model1"]
	assert.Equal(t, "iname", bm.InternalName)
	assert.Equal(t, "dname", bm.Name)
	assert.Equal(t, "active", bm.LifeCyclePhase)
	assert.Equal(t, "never", bm.TimeDeprecated)
}

func TestLoadBaseModels_MissingLocals(t *testing.T) {
	dir := t.TempDir()
	bmDir := baseModelsDir(t, dir)
	tf := `
locals {
  foo = {}
}
`
	err := os.WriteFile(filepath.Join(bmDir, "locals.tf"), []byte(tf), 0o644)
	assert.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err = LoadBaseModels(dir, env)
	assert.Error(t, err)
}
