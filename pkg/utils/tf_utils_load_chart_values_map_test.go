package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chartValuesDir(t *testing.T, base string) string {
	subdir := filepath.Join(base, "model-serving", "application", "generic_region", "model_chart_values")
	err := os.MkdirAll(subdir, 0o750) // #nosec G301
	require.NoError(t, err)
	return subdir
}

func TestLoadChartValuesMap_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	chartValuesDir(t, dir)
	// No files in subdir
	out, err := loadChartValuesMap(dir)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Empty(t, out)
}

func TestLoadChartValuesMap_ValidYaml(t *testing.T) {
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
