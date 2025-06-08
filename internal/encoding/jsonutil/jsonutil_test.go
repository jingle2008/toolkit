package jsonutil

import (
	_ "embed"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFile_JSON(t *testing.T) {
	t.Parallel()
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	_ = os.WriteFile(tmp, []byte(`{"a":42}`), 0o600) // #nosec G306
	val, err := LoadFile[Foo](tmp)
	testutil.RequireNoError(t, err)
	assert.Equal(t, 42, val.A)
}

// ---- Merged from json_utils_additional_test.go ----

func TestLoadFile_Success(t *testing.T) {
	t.Parallel()
	type sample struct {
		A int `json:"a"`
	}
	dir := t.TempDir()
	path := dir + "/sample.json"
	err := os.WriteFile(path, []byte(`{"a":42}`), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	result, err := LoadFile[sample](path)
	testutil.RequireNoError(t, err)
	assert.Equal(t, 42, result.A)
}

func TestLoadFile_UnsupportedExt(t *testing.T) {
	t.Parallel()
	type sample struct{ A int }
	dir := t.TempDir()
	path := dir + "/sample.yaml"
	err := os.WriteFile(path, []byte("a: 1"), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	_, err = LoadFile[sample](path)
	testutil.RequireError(t, err)
	assert.Contains(t, err.Error(), "extension")
}

func TestLoadFile_MissingFile(t *testing.T) {
	t.Parallel()
	type sample struct{ A int }
	dir := t.TempDir()
	path := dir + "/notfound.json"
	_, err := LoadFile[sample](path)
	testutil.RequireError(t, err)
}

func TestLoadFile_BadExt(t *testing.T) {
	t.Parallel()
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.bad"
	_ = os.WriteFile(tmp, []byte(`{"a":42}`), 0o600) // #nosec G306
	_, err := LoadFile[Foo](tmp)
	testutil.RequireError(t, err)
}

func TestLoadFile_BadJSON(t *testing.T) {
	t.Parallel()
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	_ = os.WriteFile(tmp, []byte(`{notjson}`), 0o600) // #nosec G306
	_, err := LoadFile[Foo](tmp)
	testutil.RequireError(t, err)
}

func TestPrettyJSON(t *testing.T) {
	t.Parallel()
	type Foo struct {
		A int `json:"a"`
	}
	out, err := PrettyJSON(Foo{A: 7})
	testutil.RequireNoError(t, err)
	assert.Contains(t, out, `"a": 7`)

	// error path: non-serializable value
	ch := make(chan int)
	_, err = PrettyJSON(ch)
	require.Error(t, err)
}

// ---- Merged from json_utils_additional_test.go ----

func TestPrettyJSON_Success(t *testing.T) {
	t.Parallel()
	obj := struct {
		X string `json:"x"`
		Y int    `json:"y"`
	}{"foo", 7}
	out, err := PrettyJSON(obj)
	testutil.RequireNoError(t, err)
	assert.Contains(t, out, "{\n    \"x\": \"foo\",\n    \"y\": 7\n}")
}

func TestPrettyJSON_MarshalError(t *testing.T) {
	t.Parallel()
	ch := make(chan int)
	_, err := PrettyJSON(ch)
	require.Error(t, err)
}

//go:embed testdata/sample.json
var sampleJSON []byte

func TestLoadFile_FromTestdata(t *testing.T) {
	t.Parallel()
	type sample struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/sample.json"
	testutil.RequireNoError(t, os.WriteFile(tmp, sampleJSON, 0o600))
	val, err := LoadFile[sample](tmp)
	testutil.RequireNoError(t, err)
	assert.Equal(t, 123, val.A)
}
