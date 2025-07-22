package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMetadata_JSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "meta.json")
	content := `{"tenants":[{"id":"t1","name":"Tenant1","isInternal":true}]}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	meta, err := LoadMetadata(path)
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Len(t, meta.Tenants, 1)
	assert.Equal(t, "t1", meta.Tenants[0].ID)
}

func TestLoadMetadata_YAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "meta.yaml")
	content := `
tenants:
  - id: t2
    name: Tenant2
    isInternal: false
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	meta, err := LoadMetadata(path)
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Len(t, meta.Tenants, 1)
	assert.Equal(t, "t2", meta.Tenants[0].ID)
	assert.Equal(t, "Tenant2", *meta.Tenants[0].Name)
}

func TestLoadMetadata_UnsupportedExtension(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "meta.txt")
	require.NoError(t, os.WriteFile(path, []byte("irrelevant"), 0o600))
	_, err := LoadMetadata(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported metadata file extension")
}

func TestLoadMetadata_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "meta.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: [valid"), 0o600))
	_, err := LoadMetadata(path)
	assert.Error(t, err)
}
