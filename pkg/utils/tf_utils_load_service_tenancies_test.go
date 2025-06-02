package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func shepTargetsDir(t *testing.T, base string) string {
	subdir := filepath.Join(base, "shared_modules", "shep_targets")
	err := os.MkdirAll(subdir, 0o755)
	assert.NoError(t, err)
	return subdir
}

func TestLoadServiceTenancies_Success(t *testing.T) {
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
	err := os.WriteFile(path, []byte(tf), 0o644)
	assert.NoError(t, err)

	tenancies, err := LoadServiceTenancies(dir)
	assert.NoError(t, err)
	assert.NotNil(t, tenancies)
}

func TestLoadServiceTenancies_GroupAndRegionKeys(t *testing.T) {
	dir := t.TempDir()
	subdir := shepTargetsDir(t, dir)
	tf := `
locals {
  group_foo = {}
  region_groups = {}
}
`
	path := filepath.Join(subdir, "locals.tf")
	err := os.WriteFile(path, []byte(tf), 0o644)
	assert.NoError(t, err)

	tenancies, err := LoadServiceTenancies(dir)
	assert.NoError(t, err)
	assert.NotNil(t, tenancies)
	assert.Len(t, tenancies, 0)
}

func TestLoadServiceTenancies_InvalidHCL(t *testing.T) {
	dir := t.TempDir()
	subdir := shepTargetsDir(t, dir)
	tf := `
locals {
  foo = 
`
	path := filepath.Join(subdir, "bad.tf")
	err := os.WriteFile(path, []byte(tf), 0o644)
	assert.NoError(t, err)

	_, err = LoadServiceTenancies(dir)
	assert.Error(t, err)
}
