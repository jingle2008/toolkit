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
