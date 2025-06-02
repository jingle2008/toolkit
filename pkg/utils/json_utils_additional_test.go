package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFile_Success(t *testing.T) {
	type sample struct {
		A int `json:"a"`
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	err := os.WriteFile(path, []byte(`{"a":42}`), 0o644)
	assert.NoError(t, err)

	result, err := LoadFile[sample](path)
	assert.NoError(t, err)
	assert.Equal(t, 42, result.A)
}

func TestLoadFile_UnsupportedExt(t *testing.T) {
	type sample struct{ A int }
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.yaml")
	err := os.WriteFile(path, []byte("a: 1"), 0o644)
	assert.NoError(t, err)

	_, err = LoadFile[sample](path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extension")
}

func TestLoadFile_MissingFile(t *testing.T) {
	type sample struct{ A int }
	dir := t.TempDir()
	path := filepath.Join(dir, "notfound.json")
	_, err := LoadFile[sample](path)
	assert.Error(t, err)
}

func TestPrettyJSON_Success(t *testing.T) {
	obj := struct {
		X string `json:"x"`
		Y int    `json:"y"`
	}{"foo", 7}
	out, err := PrettyJSON(obj)
	assert.NoError(t, err)
	assert.Contains(t, out, "{\n    \"x\": \"foo\",\n    \"y\": 7\n}")
}

func TestPrettyJSON_MarshalError(t *testing.T) {
	ch := make(chan int)
	_, err := PrettyJSON(ch)
	assert.Error(t, err)
}
