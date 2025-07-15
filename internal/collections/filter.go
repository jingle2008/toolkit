/*
Package collections provides generic filtering and searching utilities for filterable and named items.
*/
package collections

import (
	"strings"

	models "github.com/jingle2008/toolkit/pkg/models"
)

/*
IsMatch returns true if the item matches the filter string, optionally ignoring case.
*/
func IsMatch(item models.Filterable, filter string, ignoreCase bool) bool {
	if filter == "" {
		return true
	}

	if ignoreCase {
		filter = strings.ToLower(filter)
	}

	for _, value := range item.GetFilterableFields() {
		if value == "" {
			continue
		}

		if ignoreCase {
			value = strings.ToLower(value)
		}

		if strings.Contains(value, filter) {
			return true
		}
	}

	return false
}

/*
FilterSlice returns a slice of items that match the filter and name.
*/
/*
FilterSlice returns a slice of items that match the filter and name.
If pred is non-nil, the item must also satisfy pred(item).
*/
func FilterSlice[T models.NamedFilterable](items []T, name *string, filter string, pred func(T) bool) []T {
	var out []T
	for _, item := range items {
		if (name == nil || *name == item.GetName()) &&
			IsMatch(item, filter, true) &&
			(pred == nil || pred(item)) {
			out = append(out, item)
		}
	}
	return out
}

type kv[T any] struct {
	Key string
	Val T
}

// filterMap returns a slice of key-value pairs matching the filter and name.
func filterMap[T models.NamedFilterable](m map[string][]T, name *string, filter string, pred func(T) bool) []kv[T] {
	var out []kv[T]
	for key, value := range m {
		matchKey := strings.Contains(strings.ToLower(key), filter)
		for _, val := range value {
			if (name == nil || *name == val.GetName()) &&
				(matchKey || IsMatch(val, filter, true)) &&
				(pred == nil || pred(val)) {
				out = append(out, kv[T]{Key: key, Val: val})
			}
		}
	}
	return out
}

/*
FilterMap applies the transform function to all items in the map that match the key, name, and filter, returning a slice of results.
*/
/*
FilterMap applies the transform function to all items in the map that match the key, name, and filter, returning a slice of results.
If pred is non-nil, the item must also satisfy pred(item).
*/
func FilterMap[T models.NamedFilterable, R any](
	g map[string][]T,
	key *string,
	name *string,
	filter string,
	pred func(T) bool,
	transform func(string, T) R,
) []R {
	var results []R

	if key != nil {
		items, ok := g[*key]
		if !ok {
			return []R{}
		}

		results = make([]R, 0, len(items))

		for _, val := range FilterSlice(items, name, filter, pred) {
			results = append(results, transform(*key, val))
		}
	} else {
		for _, pair := range filterMap(g, name, filter, pred) {
			results = append(results, transform(pair.Key, pair.Val))
		}
	}

	return results
}

/*
FindByName returns a pointer to the item with the given name, or nil if not found.
*/
func FindByName[T models.NamedItem](items []T, name string) *T {
	for i := range items {
		if items[i].GetName() == name {
			return &items[i]
		}
	}

	return nil
}
