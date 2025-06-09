package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	name  string
	value string
}

func (t testStruct) GetName() string               { return t.name }
func (t testStruct) GetFilterableFields() []string { return []string{t.name, t.value} }

func TestFilterSlice_Basic(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}, {"baz", "qux"}}
	name := "foo"
	filter := ""
	var out []testStruct
	FilterSlice(items, &name, filter, func(_ int, item testStruct) bool {
		out = append(out, item)
		return true
	})
	assert.Len(t, out, 1)
	assert.Equal(t, "foo", out[0].name)
}

func TestFilterSlice_Empty(t *testing.T) {
	t.Parallel()
	items := []testStruct{}
	var out []testStruct
	FilterSlice(items, nil, "", func(_ int, item testStruct) bool {
		out = append(out, item)
		return true
	})
	assert.Empty(t, out)
}

func TestFilterSlice_CaseInsensitive(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"Foo", "Bar"}, {"baz", "qux"}}
	var out []testStruct
	FilterSlice(items, nil, "foo", func(_ int, item testStruct) bool {
		out = append(out, item)
		return true
	})
	assert.Len(t, out, 1)
	assert.Equal(t, "Foo", out[0].name)
}

func TestFilterSlice_EmptyFilterReturnsAll(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"a", "b"}, {"c", "d"}}
	var out []testStruct
	FilterSlice(items, nil, "", func(_ int, item testStruct) bool {
		out = append(out, item)
		return true
	})
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
	out := FilterMap(m, nil, nil, "", func(_ string, item testStruct) testStruct { return item })
	assert.Len(t, out, 2)
}

func TestFilterMap_Empty(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{}
	out := FilterMap(m, nil, nil, "", func(_ string, item testStruct) testStruct { return item })
	assert.Empty(t, out)
}

// Additional edge-case tests for coverage
func TestFilterSlice_AllFilteredOut(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}, {"baz", "qux"}}
	var out []testStruct
	FilterSlice(items, nil, "notfound", func(_ int, item testStruct) bool {
		out = append(out, item)
		return true
	})
	assert.Empty(t, out)
}

func TestFilterSlice_NilPredicate(t *testing.T) {
	t.Parallel()
	items := []testStruct{{"foo", "bar"}}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic with nil predicate")
		}
	}()
	FilterSlice(items, nil, "", nil)
}

func TestFilterMap_NilMap(t *testing.T) {
	t.Parallel()
	var m map[string][]testStruct
	out := FilterMap(m, nil, nil, "", func(_ string, item testStruct) testStruct { return item })
	assert.Empty(t, out)
}

func TestFilterMap_AllFilteredOut(t *testing.T) {
	t.Parallel()
	m := map[string][]testStruct{
		"a": {{"foo", "bar"}},
	}
	out := FilterMap(m, nil, nil, "notfound", func(_ string, item testStruct) testStruct { return item })
	assert.Empty(t, out)
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
	items := make([]testStruct, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = testStruct{name: "foo", value: "bar"}
	}
	name := "foo"
	filter := ""
	for n := 0; n < b.N; n++ {
		var out []testStruct
		FilterSlice(items, &name, filter, func(_ int, item testStruct) bool {
			out = append(out, item)
			return true
		})
	}
}
