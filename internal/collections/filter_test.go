package collections

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	name  string
	value string
}

func (t testStruct) GetName() string               { return t.name }
func (t testStruct) GetFilterableFields() []string { return []string{t.name, t.value} }
func (testStruct) IsFaulty() bool                  { return false }

func TestFilterSlice_Basic(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}, {"baz", "qux"}}
	name := "foo"
	filter := ""
	out := FilterSlice(items, &name, filter, nil)
	assert.Len(t, out, 1)
	assert.Equal(t, "foo", out[0].name)
}

func TestFilterSlice_Empty(t *testing.T) {
	t.Parallel()
	items := []testStruct{}
	out := FilterSlice(items, nil, "", nil)
	assert.Empty(t, out)
}

func TestFilterSlice_CaseInsensitive(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"Foo", "Bar"}, {"baz", "qux"}}
	out := FilterSlice(items, nil, "foo", nil)
	assert.Len(t, out, 1)
	assert.Equal(t, "Foo", out[0].name)
}

func TestFilterSlice_EmptyFilterReturnsAll(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"a", "b"}, {"c", "d"}}
	out := FilterSlice(items, nil, "", nil)
	assert.Len(t, out, 2)
}

func TestFindByName(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}, {"baz", "qux"}}
	ptr := FindByName(items, "baz")
	assert.NotNil(t, ptr)
	assert.Equal(t, "baz", ptr.name)
	ptr = FindByName(items, "nope")
	assert.Nil(t, ptr)
}

func TestIsMatch(t *testing.T) {
	t.Parallel()
	item := testStruct{"foo", "bar"}
	assert.True(t, IsMatch(item, "foo", false))
	assert.True(t, IsMatch(item, "bar", false))
	assert.False(t, IsMatch(item, "baz", false))
	assert.True(t, IsMatch(item, "FOO", true))
	assert.False(t, IsMatch(item, "FOO", false))
}

func TestFilterMap_Basic(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}},
		"b": {{"baz", "qux"}},
	}
	out := FilterMap(m, nil, nil, "", nil)
	assert.Len(t, out, 2)
}

func TestFilterMap_Empty(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{}
	out := FilterMap(m, nil, nil, "", nil)
	assert.Empty(t, out)
}

func TestFilterSlice_AllFilteredOut(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}, {"baz", "qux"}}
	out := FilterSlice(items, nil, "notfound", nil)
	assert.Empty(t, out)
}

func TestFilterMap_NilMap(t *testing.T) {
	t.Parallel()
	var m map[string][]testStruct
	out := FilterMap(m, nil, nil, "", nil)
	assert.Empty(t, out)
}

func TestFilterMap_AllFilteredOut(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}},
	}
	out := FilterMap(m, nil, nil, "notfound", nil)
	assert.Empty(t, out)
}

func TestFilterMap_WithKey(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}, {"baz", "qux"}},
		"b": {{"quux", "corge"}},
	}
	key := "a"
	out := FilterMap(m, &key, nil, "", nil)
	assert.Len(t, out, 1)
	assert.Contains(t, out, "a")
	assert.Len(t, out["a"], 2)
	names := []string{out["a"][0].name, out["a"][1].name}
	assert.ElementsMatch(t, []string{"foo", "baz"}, names)
}

func TestFilterMap_WithKeyNotFound(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}},
	}
	key := "b"
	out := FilterMap(m, &key, nil, "", nil)
	assert.Empty(t, out)
}

func TestFilterMap_WithKeyAndName(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}, {"baz", "qux"}},
	}
	key := "a"
	name := "baz"
	out := FilterMap(m, &key, &name, "", nil)
	assert.Len(t, out, 1)
	assert.Contains(t, out, "a")
	assert.Len(t, out["a"], 1)
	assert.Equal(t, "baz", out["a"][0].name)
}

func TestFilterMap_WithKeyAndFilter(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}, {"baz", "qux"}},
	}
	key := "a"
	out := FilterMap(m, &key, nil, "qux", nil)
	assert.Len(t, out, 1)
	assert.Contains(t, out, "a")
	assert.Len(t, out["a"], 1)
	assert.Equal(t, "baz", out["a"][0].name)
}

func TestFindByName_EmptySlice(t *testing.T) {
	t.Parallel()
	var items []testStruct
	ptr := FindByName(items, "foo")
	assert.Nil(t, ptr)
}

func TestIsMatch_EmptyFields(t *testing.T) {
	t.Parallel()
	item := testStruct{"", ""}
	assert.False(t, IsMatch(item, "foo", false))
	assert.True(t, IsMatch(item, "", false)) // empty filter should match
}

func BenchmarkFilterSlice(b *testing.B) {
	data := make([]testStruct, 1000)
	for i := range data {
		data[i] = testStruct{
			name:  fmt.Sprintf("item-%d", i),
			value: fmt.Sprintf("%d", i),
		}
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = FilterSlice(data, nil, "item-5", nil)
		}
	})
}
