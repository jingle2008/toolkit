// Package collections provides generic collection utilities for filtering and sorting.
package collections

import "sort"

// SortKeyedItems sorts a slice of items implementing GetKey() by key in ascending order.
// T must implement GetKey() string.
func SortKeyedItems[T interface{ GetKey() string }](items []T) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetKey() < items[j].GetKey()
	})
}

// SortNamedItems sorts a slice of items implementing GetName() by name in ascending order.
// T must implement GetName() string.
func SortNamedItems[T interface{ GetName() string }](items []T) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})
}
