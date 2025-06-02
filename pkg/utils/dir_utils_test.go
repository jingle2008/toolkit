package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	// create a .txt file and a .go file
	txtFile := dir + "/foo.txt"
	goFile := dir + "/bar.go"
	os.WriteFile(txtFile, []byte("x"), 0644)
	os.WriteFile(goFile, []byte("y"), 0644)

	files, err := ListFiles(dir, ".txt")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(files))
	assert.Contains(t, files[0], "foo.txt")

	// error path: non-existent dir
	_, err = ListFiles(dir+"/nope", ".txt")
	assert.Error(t, err)
}
