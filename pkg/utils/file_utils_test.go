package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	allowExt := map[string]struct{}{".txt": {}, ".json": {}}
	txtFile := filepath.Join(tmpDir, "a.txt")
	jsonFile := filepath.Join(tmpDir, "b.json")
	mdFile := filepath.Join(tmpDir, "c.md")

	_ = os.WriteFile(txtFile, []byte("foo"), 0o600)          // #nosec G306
	_ = os.WriteFile(jsonFile, []byte("{\"bar\":1}"), 0o600) // #nosec G306
	_ = os.WriteFile(mdFile, []byte("# md"), 0o600)          // #nosec G306

	// Allowed extension
	data, err := SafeReadFile(txtFile, tmpDir, allowExt)
	require.NoError(t, err)
	assert.Equal(t, []byte("foo"), data)

	// Disallowed extension
	_, err = SafeReadFile(mdFile, tmpDir, allowExt)
	require.Error(t, err)

	// File outside baseDir
	outsideFile := filepath.Join(os.TempDir(), "outside.txt")
	_ = os.WriteFile(outsideFile, []byte("bad"), 0o600) // #nosec G306
	_, err = SafeReadFile(outsideFile, tmpDir, allowExt)
	require.Error(t, err)

	// Clean up
	_ = os.Remove(outsideFile)
}

func TestSafeReadFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.json")
	err := os.WriteFile(path, []byte(`{"ok":true}`), 0o600) // #nosec G306
	require.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	data, err := SafeReadFile(path, dir, allow)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(data))
}

func TestSafeReadFile_DisallowedExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	err := os.WriteFile(path, []byte("bad"), 0o600) // #nosec G306
	require.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extension")
}

func TestSafeReadFile_DirTraversal(t *testing.T) {
	dir := t.TempDir()
	allow := map[string]struct{}{".json": {}}
	evil := filepath.Join(dir, "..", "evil.json")
	_, err := SafeReadFile(evil, dir, allow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access outside trusted dir")
}

func TestSafeReadFile_OutsideBaseDir(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	path := filepath.Join(otherDir, "foo.json")
	err := os.WriteFile(path, []byte("{}"), 0o600) // #nosec G306
	require.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access outside trusted dir")
}
