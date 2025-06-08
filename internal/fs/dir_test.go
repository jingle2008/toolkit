package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
)

func TestListFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// create a .txt file and a .go file
	txtFile := dir + "/foo.txt"
	goFile := dir + "/bar.go"
	_ = os.WriteFile(txtFile, []byte("x"), 0o600) // #nosec G306
	_ = os.WriteFile(goFile, []byte("y"), 0o600)  // #nosec G306

	files, err := ListFiles(dir, ".txt")
	testutil.RequireNoError(t, err)
	testutil.Equal(t, 1, len(files))
	testutil.Contains(t, files[0], "foo.txt")

	// error path: non-existent dir
	_, err = ListFiles(dir+"/nope", ".txt")
	testutil.RequireError(t, err)
}

func TestListFiles_MatchExt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := []string{"a.go", "b.txt", "c.go"}
	for _, name := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600) // #nosec G306
		testutil.RequireNoError(t, err)
	}
	out, err := ListFiles(dir, ".go")
	testutil.RequireNoError(t, err)
	testutil.Equal(t, 2, len(out))
	testutil.Contains(t, out[0], ".go")
	testutil.Contains(t, out[1], ".go")
}

func TestListFiles_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := ListFiles(dir, ".json")
	testutil.RequireNoError(t, err)
}

func TestListFiles_NonExistentDir(t *testing.T) {
	t.Parallel()
	_, err := ListFiles("/no/such/dir", ".go")
	testutil.RequireError(t, err)
}
