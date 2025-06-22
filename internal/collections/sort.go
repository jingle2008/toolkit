// Package collections provides generic collection utilities for filtering and sorting.
package collections

// Package collections provides generic collection utilities for filtering and sorting.
import (
	"slices"
)

// SortKeyedItems sorts a slice of items implementing GetKey() by key in ascending order.
// T must implement GetKey() string.
func SortKeyedItems[T interface{ GetKey() string }](items []T) {
	slices.SortFunc(items, func(a, b T) int {
		if a.GetKey() < b.GetKey() {
			return -1
		}
		if a.GetKey() > b.GetKey() {
			return 1
		}
		return 0
	})
}

// SortNamedItems sorts a slice of items implementing GetName() by name in ascending order.
// T must implement GetName() string.
func SortNamedItems[T interface{ GetName() string }](items []T) {
	slices.SortFunc(items, func(a, b T) int {
		if a.GetName() < b.GetName() {
			return -1
		}
		if a.GetName() > b.GetName() {
			return 1
		}
		return 0
	})
}
