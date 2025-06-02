package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	allowExt := map[string]struct{}{".txt": {}, ".json": {}}
	txtFile := filepath.Join(tmpDir, "a.txt")
	jsonFile := filepath.Join(tmpDir, "b.json")
	mdFile := filepath.Join(tmpDir, "c.md")

	os.WriteFile(txtFile, []byte("foo"), 0o644)
	os.WriteFile(jsonFile, []byte("{\"bar\":1}"), 0o644)
	os.WriteFile(mdFile, []byte("# md"), 0o644)

	// Allowed extension
	data, err := SafeReadFile(txtFile, tmpDir, allowExt)
	assert.NoError(t, err)
	assert.Equal(t, []byte("foo"), data)

	// Disallowed extension
	_, err = SafeReadFile(mdFile, tmpDir, allowExt)
	assert.Error(t, err)

	// File outside baseDir
	outsideFile := filepath.Join(os.TempDir(), "outside.txt")
	os.WriteFile(outsideFile, []byte("bad"), 0o644)
	_, err = SafeReadFile(outsideFile, tmpDir, allowExt)
	assert.Error(t, err)

	// Clean up
	os.Remove(outsideFile)
}
