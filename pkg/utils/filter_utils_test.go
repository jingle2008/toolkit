package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Name  string
	Value int
}

func (t testStruct) GetName() string {
	return t.Name
}

func (t testStruct) GetFilterableFields() []string {
	return []string{t.Name}
}

func TestFilterSlice_Basic(t *testing.T) {
	input := []testStruct{
		{Name: "foo", Value: 1},
		{Name: "bar", Value: 2},
		{Name: "baz", Value: 3},
	}
	var out []testStruct
	FilterSlice(input, nil, "ba", func(_ int, v testStruct) bool {
		out = append(out, v)
		return true
	})
	assert.Len(t, out, 2)
	assert.Equal(t, "bar", out[0].Name)
	assert.Equal(t, "baz", out[1].Name)
}

func TestFilterSlice_Empty(t *testing.T) {
	var input []testStruct
	var out []testStruct
	FilterSlice(input, nil, "foo", func(_ int, v testStruct) bool {
		out = append(out, v)
		return true
	})
	assert.Empty(t, out)
}

func TestFindByName(t *testing.T) {
	input := []testStruct{
		{Name: "foo", Value: 1},
		{Name: "bar", Value: 2},
	}
	res := FindByName(input, "bar")
	assert.NotNil(t, res)
	assert.Equal(t, "bar", (*res).GetName())
	assert.Nil(t, FindByName(input, "baz"))
}

func TestIsMatch(t *testing.T) {
	obj := testStruct{Name: "hello"}
	assert.True(t, IsMatch(obj, "hell", true))
	assert.False(t, IsMatch(obj, "world", true))
}

func TestFilterMap_Basic(t *testing.T) {
	m := map[string][]testStruct{
		"foo": {{Name: "foo", Value: 1}},
		"bar": {{Name: "bar", Value: 2}},
	}
	var out []string
	FilterMap(m, nil, nil, "foo", func(s string, v testStruct) interface{} {
		out = append(out, s)
		return nil
	})
	assert.Contains(t, out, "foo")
	assert.NotContains(t, out, "bar")
}
