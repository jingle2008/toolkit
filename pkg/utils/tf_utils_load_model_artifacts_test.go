package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func tensorrtModelsDir(t *testing.T, base string) string {
	dir := filepath.Join(base, "shared_modules", "tensorrt_models_config")
	err := os.MkdirAll(dir, 0o755)
	assert.NoError(t, err)
	return dir
}

func TestLoadModelArtifacts_Success(t *testing.T) {
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
	err := os.WriteFile(filepath.Join(subdir, "locals.tf"), []byte(tf), 0o644)
	assert.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	arts, err := LoadModelArtifacts(dir, env)
	assert.NoError(t, err)
	assert.NotNil(t, arts)
	assert.True(t, len(arts) >= 1)
}

func TestLoadModelArtifacts_MissingMap(t *testing.T) {
	dir := t.TempDir()
	subdir := tensorrtModelsDir(t, dir)
	tf := `
locals {
  foo = {}
}
`
	err := os.WriteFile(filepath.Join(subdir, "locals.tf"), []byte(tf), 0o644)
	assert.NoError(t, err)
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	_, err = LoadModelArtifacts(dir, env)
	assert.Error(t, err)
}
