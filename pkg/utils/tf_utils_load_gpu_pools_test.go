package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func poolsConfigDir(t *testing.T, base, sub string) string {
	dir := filepath.Join(base, "shared_modules", sub)
	err := os.MkdirAll(dir, 0o755)
	assert.NoError(t, err)
	return dir
}

func TestLoadGpuPools_Success(t *testing.T) {
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
	err := os.WriteFile(filepath.Join(ipDir, "locals.tf"), []byte(tf), 0o644)
	assert.NoError(t, err)

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
	err = os.WriteFile(filepath.Join(cnDir, "locals.tf"), []byte(tf2), 0o644)
	assert.NoError(t, err)

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
	err = os.WriteFile(filepath.Join(okeDir, "locals.tf"), []byte(tf3), 0o644)
	assert.NoError(t, err)

	pools, err := LoadGpuPools(dir, env)
	assert.NoError(t, err)
	assert.NotNil(t, pools)
	assert.True(t, len(pools) >= 3)
}

func TestLoadGpuPools_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	env := models.Environment{Realm: "test", Type: "dev", Region: "us-test-1"}
	// No config files
	_, err := LoadGpuPools(dir, env)
	assert.Error(t, err)
}
