package utils

import (
	"testing"
)

type fuzzStruct struct {
	name  string
	value string
}

func (f fuzzStruct) GetName() string               { return f.name }
func (f fuzzStruct) GetFilterableFields() []string { return []string{f.name, f.value} }

func FuzzFilterMap(f *testing.F) {
	m := map[string][]fuzzStruct{
		"a": {{"foo", "bar"}, {"baz", "qux"}},
		"b": {{"quux", "corge"}},
	}
	f.Add("foo")
	f.Add("baz")
	f.Add("")
	f.Fuzz(func(t *testing.T, filter string) {
		_ = FilterMap(m, nil, nil, filter, func(_ string, item fuzzStruct) fuzzStruct { return item })
	})
}

func FuzzFilterSlice(f *testing.F) {
	items := []fuzzStruct{{"foo", "bar"}, {"baz", "qux"}, {"quux", "corge"}}
	f.Add("foo")
	f.Add("baz")
	f.Add("")
	f.Fuzz(func(t *testing.T, filter string) {
		var out []fuzzStruct
		FilterSlice(items, nil, filter, func(i int, item fuzzStruct) bool {
			out = append(out, item)
			return true
		})
	})
}
