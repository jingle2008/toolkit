package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListFiles_MatchExt(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.go", "b.txt", "c.go"}
	for _, name := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)
		assert.NoError(t, err)
	}
	out, err := ListFiles(dir, ".go")
	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.Contains(t, out[0], ".go")
	assert.Contains(t, out[1], ".go")
}

func TestListFiles_NoMatch(t *testing.T) {
	dir := t.TempDir()
	_, err := ListFiles(dir, ".json")
	assert.NoError(t, err)
}

func TestListFiles_NonExistentDir(t *testing.T) {
	_, err := ListFiles("/no/such/dir", ".go")
	assert.Error(t, err)
}
