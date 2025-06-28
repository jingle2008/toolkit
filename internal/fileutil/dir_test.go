package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// create a .txt file and a .go file
	txtFile := dir + "/foo.txt"
	goFile := dir + "/bar.go"
	_ = os.WriteFile(txtFile, []byte("x"), 0o600) // #nosec G306
	_ = os.WriteFile(goFile, []byte("y"), 0o600)  // #nosec G306

	files, err := ListFiles(context.Background(), dir, ".txt")
	require.NoError(t, err)
	assert.Equal(t, 1, len(files))
	assert.Contains(t, files[0], "foo.txt")

	// error path: non-existent dir
	_, err = ListFiles(context.Background(), dir+"/nope", ".txt")
	require.Error(t, err)
}

func TestListFiles_MatchExt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := []string{"a.go", "b.txt", "c.go"}
	for _, name := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600) // #nosec G306
		require.NoError(t, err)
	}
	out, err := ListFiles(context.Background(), dir, ".go")
	require.NoError(t, err)
	assert.Equal(t, 2, len(out))
	assert.Contains(t, out[0], ".go")
	assert.Contains(t, out[1], ".go")
}

func TestListFiles_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := ListFiles(context.Background(), dir, ".json")
	require.NoError(t, err)
}

func TestListFiles_NonExistentDir(t *testing.T) {
	t.Parallel()
	_, err := ListFiles(context.Background(), "/no/such/dir", ".go")
	require.Error(t, err)
}
