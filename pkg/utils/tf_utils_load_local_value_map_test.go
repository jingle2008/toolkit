package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTfFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600) // #nosec G306
	require.NoError(t, err)
	return path
}

func TestLoadLocalValueMap_Success(t *testing.T) {
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
