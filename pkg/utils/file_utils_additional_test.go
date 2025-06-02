package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeReadFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.json")
	err := os.WriteFile(path, []byte(`{"ok":true}`), 0o644)
	assert.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	data, err := SafeReadFile(path, dir, allow)
	assert.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(data))
}

func TestSafeReadFile_DisallowedExt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	err := os.WriteFile(path, []byte("bad"), 0o644)
	assert.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extension")
}

func TestSafeReadFile_DirTraversal(t *testing.T) {
	dir := t.TempDir()
	allow := map[string]struct{}{".json": {}}
	evil := filepath.Join(dir, "..", "evil.json")
	_, err := SafeReadFile(evil, dir, allow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access outside trusted dir")
}

func TestSafeReadFile_OutsideBaseDir(t *testing.T) {
	dir := t.TempDir()
	otherDir := t.TempDir()
	path := filepath.Join(otherDir, "foo.json")
	err := os.WriteFile(path, []byte("{}"), 0o644)
	assert.NoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access outside trusted dir")
}
