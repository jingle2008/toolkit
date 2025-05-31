package utils

import (
	"sort"
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
		if ignoreCase {
			value = strings.ToLower(value)
		}

		if strings.Contains(value, filter) {
			return true
		}
	}

	return false
}

// perform action on items in slice that match the filter and name
// return false in action to stop further processing
func FilterSlice[T models.NamedFilterable](items []T, name *string, filter string,
	action func(int, T) bool,
) {
	idx := 0
	for _, item := range items {
		if (name == nil || *name == item.GetName()) && IsMatch(item, filter, true) {
			if !action(idx, item) {
				return
			}

			idx += 1
		}
	}
}

func getSortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

// perform action on items in map that match the filter and name
// return false in action to stop further processing
func filterMap[T models.NamedFilterable](m map[string][]T, name *string,
	filter string, action func(int, string, T) bool,
) {
	idx, keys := 0, getSortedKeys(m)

	for _, key := range keys {
		matchKey := strings.Contains(strings.ToLower(key), filter)

		for _, val := range m[key] {
			if (name == nil || *name == val.GetName()) &&
				(matchKey || IsMatch(val, filter, true)) {
				if !action(idx, key, val) {
					return
				}

				idx += 1
			}
		}
	}
}

/*
FilterMap applies the transform function to all items in the map that match the key, name, and filter, returning a slice of results.
*/
func FilterMap[T models.NamedFilterable, R any](g map[string][]T,
	key *string, name *string, filter string, transform func(string, T) R,
) []R {
	var results []R

	if key != nil {
		items, ok := g[*key]
		if !ok {
			return []R{}
		}

		results = make([]R, 0, len(items))

		FilterSlice(items, name, filter, func(_ int, val T) bool {
			results = append(results, transform(*key, val))
			return true
		})
	} else {
		filterMap(g, name, filter, func(_ int, key string, val T) bool {
			results = append(results, transform(key, val))
			return true
		})
	}

	return results
}

/*
FindByName returns a pointer to the item with the given name, or nil if not found.
*/
func FindByName[T models.NamedItem](items []T, name string) *T {
	for _, item := range items {
		if item.GetName() == name {
			return &item
		}
	}

	return nil
}
