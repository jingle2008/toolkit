package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	// Create files
	f1 := filepath.Join(tmpDir, "a.txt")
	f2 := filepath.Join(tmpDir, "b.txt")
	f3 := filepath.Join(tmpDir, "c.go")
	os.WriteFile(f1, []byte("foo"), 0644)
	os.WriteFile(f2, []byte("bar"), 0644)
	os.WriteFile(f3, []byte("baz"), 0644)

	files, err := ListFiles(tmpDir, ".txt")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{f1, f2}, files)

	filesGo, err := ListFiles(tmpDir, ".go")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{f3}, filesGo)

	filesNone, err := ListFiles(tmpDir, ".md")
	assert.NoError(t, err)
	assert.Empty(t, filesNone)
}
