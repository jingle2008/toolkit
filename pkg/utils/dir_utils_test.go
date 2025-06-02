package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	// create a .txt file and a .go file
	txtFile := dir + "/foo.txt"
	goFile := dir + "/bar.go"
	_ = os.WriteFile(txtFile, []byte("x"), 0o600) // #nosec G306
	_ = os.WriteFile(goFile, []byte("y"), 0o600)  // #nosec G306

	files, err := ListFiles(dir, ".txt")
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "foo.txt")

	// error path: non-existent dir
	_, err = ListFiles(dir+"/nope", ".txt")
	assert.Error(t, err)
}
