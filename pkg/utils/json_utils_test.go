package utils_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jingle2008/toolkit/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestLoadFile_JSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	os.WriteFile(tmp, []byte(`{"a":42}`), 0o644)
	val, err := utils.LoadFile[Foo](tmp)
	assert.NoError(t, err)
	assert.Equal(t, 42, val.A)
}

// ---- Merged from json_utils_additional_test.go ----

func TestLoadFile_Success(t *testing.T) {
	type sample struct {
		A int `json:"a"`
	}
	dir := t.TempDir()
	path := dir + "/sample.json"
	err := os.WriteFile(path, []byte(`{"a":42}`), 0o644)
	assert.NoError(t, err)

	result, err := utils.LoadFile[sample](path)
	assert.NoError(t, err)
	assert.Equal(t, 42, result.A)
}

func TestLoadFile_UnsupportedExt(t *testing.T) {
	type sample struct{ A int }
	dir := t.TempDir()
	path := dir + "/sample.yaml"
	err := os.WriteFile(path, []byte("a: 1"), 0o644)
	assert.NoError(t, err)

	_, err = utils.LoadFile[sample](path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extension")
}

func TestLoadFile_MissingFile(t *testing.T) {
	type sample struct{ A int }
	dir := t.TempDir()
	path := dir + "/notfound.json"
	_, err := utils.LoadFile[sample](path)
	assert.Error(t, err)
}

func TestLoadFile_BadExt(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.bad"
	os.WriteFile(tmp, []byte(`{"a":42}`), 0o644)
	_, err := utils.LoadFile[Foo](tmp)
	assert.Error(t, err)
}

func TestLoadFile_BadJSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	os.WriteFile(tmp, []byte(`{notjson}`), 0o644)
	_, err := utils.LoadFile[Foo](tmp)
	assert.Error(t, err)
}

func TestPrettyJSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	out, err := utils.PrettyJSON(Foo{A: 7})
	assert.NoError(t, err)
	assert.Contains(t, out, `"a": 7`)

	// error path: non-serializable value
	ch := make(chan int)
	_, err = utils.PrettyJSON(ch)
	assert.Error(t, err)
}

// ---- Merged from json_utils_additional_test.go ----

func TestPrettyJSON_Success(t *testing.T) {
	obj := struct {
		X string `json:"x"`
		Y int    `json:"y"`
	}{"foo", 7}
	out, err := utils.PrettyJSON(obj)
	assert.NoError(t, err)
	assert.Contains(t, out, "{\n    \"x\": \"foo\",\n    \"y\": 7\n}")
}

func TestPrettyJSON_MarshalError(t *testing.T) {
	ch := make(chan int)
	_, err := utils.PrettyJSON(ch)
	assert.Error(t, err)
}

func TestLoadFile_FromTestdata(t *testing.T) {
	type sample struct {
		A int `json:"a"`
	}
	_, filename, _, _ := runtime.Caller(0)
	testdataPath := filepath.Join(filepath.Dir(filename), "testdata", "sample.json")
	val, err := utils.LoadFile[sample](testdataPath)
	assert.NoError(t, err)
	assert.Equal(t, 123, val.A)
}
