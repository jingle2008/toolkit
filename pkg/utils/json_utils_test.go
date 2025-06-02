package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFile_JSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	os.WriteFile(tmp, []byte(`{"a":42}`), 0o644)
	val, err := LoadFile[Foo](tmp)
	assert.NoError(t, err)
	assert.Equal(t, 42, val.A)
}

func TestLoadFile_BadExt(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.bad"
	os.WriteFile(tmp, []byte(`{"a":42}`), 0o644)
	_, err := LoadFile[Foo](tmp)
	assert.Error(t, err)
}

func TestLoadFile_BadJSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	tmp := t.TempDir() + "/foo.json"
	os.WriteFile(tmp, []byte(`{notjson}`), 0o644)
	_, err := LoadFile[Foo](tmp)
	assert.Error(t, err)
}

func TestPrettyJSON(t *testing.T) {
	type Foo struct {
		A int `json:"a"`
	}
	out, err := PrettyJSON(Foo{A: 7})
	assert.NoError(t, err)
	assert.Contains(t, out, `"a": 7`)

	// error path: non-serializable value
	ch := make(chan int)
	_, err = PrettyJSON(ch)
	assert.Error(t, err)
}
