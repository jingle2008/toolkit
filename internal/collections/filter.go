/*
Package collections provides generic filtering and searching utilities for filterable and named items.
*/
package collections

import (
	"strings"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// TruncateSlice returns items[:limit] when limit > 0 and the slice has
// more items than the limit; otherwise returns items unchanged. A
// non-positive limit means "no limit" (matches kubectl --limit=0
// convention) so callers can pass an unbound limit through without a
// special branch.
//
// Truncation always happens AFTER filtering: with the toolkit's fuzzy
// substring filter being client-side, "filter + limit" only makes
// sense as "first N matching the filter". Source-side `ListOptions.Limit`
// would silently break that contract when filter is set (the K8s API
// returns the first N by its own ordering, only some of which match
// the filter).
func TruncateSlice[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

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

// FilterMap returns a map of filtered items from the input map, filtered by key, name, filter, and an optional predicate.
//
//nolint:cyclop // function is clear and further splitting would reduce readability
func FilterMap[T models.NamedFilterable](
	g map[string][]T,
	key *string,
	name *string,
	filter string,
	pred func(T) bool,
) map[string][]T {
	var results map[string][]T
	if key != nil {
		if items, ok := g[*key]; ok {
			results = map[string][]T{*key: FilterSlice(items, name, filter, pred)}
		}
	} else {
		results = make(map[string][]T)
		for key, value := range g {
			matchKey := strings.Contains(strings.ToLower(key), filter)
			for _, val := range value {
				if (name == nil || *name == val.GetName()) &&
					(matchKey || IsMatch(val, filter, true)) &&
					(pred == nil || pred(val)) {
					results[key] = append(results[key], val)
				}
			}
		}
	}

	return results
}

// FilterMapOrAll returns g unchanged when filter is empty, and a
// filtered copy otherwise. Useful for the common case where callers
// want "all rows when no filter is set" without conditional plumbing
// at every callsite.
func FilterMapOrAll[T models.NamedFilterable](g map[string][]T, filter string) map[string][]T {
	if filter == "" {
		return g
	}
	return FilterMap(g, nil, nil, filter, nil)
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
