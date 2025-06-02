package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func chartValuesDir(t *testing.T, base string) string {
	subdir := filepath.Join(base, "model-serving", "application", "generic_region", "model_chart_values")
	err := os.MkdirAll(subdir, 0o755)
	assert.NoError(t, err)
	return subdir
}

func TestLoadChartValuesMap_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	chartValuesDir(t, dir)
	// No files in subdir
	out, err := loadChartValuesMap(dir)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out, 0)
}

func TestLoadChartValuesMap_ValidYaml(t *testing.T) {
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	content := `
model:
  name: "test"
`
	path := filepath.Join(subdir, "foo.yaml")
	err := os.WriteFile(path, []byte(content), 0o644)
	assert.NoError(t, err)

	out, err := loadChartValuesMap(dir)
	assert.NoError(t, err)
	assert.Contains(t, out, "foo.yaml")
}

func TestLoadChartValuesMap_SafeReadFileError(t *testing.T) {
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	// Create a file and remove read permissions
	path := filepath.Join(subdir, "bad.yaml")
	err := os.WriteFile(path, []byte("foo: bar"), 0o000)
	assert.NoError(t, err)
	defer os.Chmod(path, 0o644) // restore permissions for cleanup

	out, err := loadChartValuesMap(dir)
	assert.NoError(t, err)
	// Should skip the unreadable file, so out is empty
	assert.Len(t, out, 0)
}

func TestLoadChartValuesMap_InvalidYaml(t *testing.T) {
	dir := t.TempDir()
	subdir := chartValuesDir(t, dir)
	path := filepath.Join(subdir, "bad.yaml")
	err := os.WriteFile(path, []byte("foo: [unclosed"), 0o644)
	assert.NoError(t, err)

	out, err := loadChartValuesMap(dir)
	assert.NoError(t, err)
	// Should skip the invalid file, so out is empty
	assert.Len(t, out, 0)
}
