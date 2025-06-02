package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	type Foo struct {
		Bar int `json:"bar"`
	}
	path := filepath.Join(tmpDir, "foo.json")
	os.WriteFile(path, []byte(`{"bar":42}`), 0644)

	obj, err := LoadFile[Foo](path)
	assert.NoError(t, err)
	assert.Equal(t, 42, obj.Bar)
}

func TestLoadFile_BadExt(t *testing.T) {
	tmpDir := t.TempDir()
	type Foo struct {
		Bar int `json:"bar"`
	}
	path := filepath.Join(tmpDir, "foo.txt")
	os.WriteFile(path, []byte(`{"bar":42}`), 0644)

	_, err := LoadFile[Foo](path)
	assert.Error(t, err)
}

func TestPrettyJSON(t *testing.T) {
	type Foo struct {
		Bar int `json:"bar"`
	}
	obj := Foo{Bar: 7}
	out, err := PrettyJSON(obj)
	assert.NoError(t, err)
	assert.Contains(t, out, "\"bar\": 7")
}
