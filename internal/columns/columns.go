// Package columns is the single source of truth for the columns
// rendered by `toolkit get` (CLI table/csv/tsv) and the TUI table
// view. One Set or GroupedSet is defined per domain.Category; both
// surfaces consume them through adapters.
package columns

import "strings"

// Column is a column for a flat (non-grouped) category.
//
// TruncateMiddle is an optional rendering hint: when the cell value is
// wider than the column at display time, the TUI elides the MIDDLE
// (head + "…" + tail) rather than chopping the tail. Useful for
// OCID-suffix-shaped values where the head identifies the resource
// shape and the tail is the distinguishing portion. CLI surfaces
// ignore this hint — they emit the full value.
type Column[T any] struct {
	Title          string
	Key            string
	Ratio          float64
	Render         func(T) string
	TruncateMiddle bool
}

// GroupedColumn is a column for a grouped category (loader returns
// map[string][]T). Render receives both the group key and the item;
// any column can use either. A "group key column" is just a
// GroupedColumn whose Render ignores `item` and returns `key`.
//
// TruncateMiddle has the same semantics as Column.TruncateMiddle.
type GroupedColumn[T any] struct {
	Title          string
	Key            string
	Ratio          float64
	Render         func(key string, item T) string
	TruncateMiddle bool
}

// Set is the canonical column list for a flat category.
type Set[T any] struct {
	Columns []Column[T]
}

// GroupedSet is the canonical column list for a grouped category.
type GroupedSet[T any] struct {
	Columns []GroupedColumn[T]
}

// SelectColumns returns the columns of s whose Key is in keys,
// in the order given by keys. Returns an error listing all unknown
// keys (so the CLI can show a single complete message).
func (s Set[T]) SelectColumns(keys []string) ([]Column[T], error) {
	byKey := make(map[string]Column[T], len(s.Columns))
	for _, c := range s.Columns {
		byKey[c.Key] = c
	}
	out := make([]Column[T], 0, len(keys))
	var unknown []string
	for _, k := range keys {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		} else {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, &UnknownColumnError{Unknown: unknown, Valid: s.Keys()}
	}
	return out, nil
}

// Keys returns the keys declared on s in order.
func (s Set[T]) Keys() []string {
	out := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		out[i] = c.Key
	}
	return out
}

// Titles returns the Title for each column of s in declared order.
func (s Set[T]) Titles() []string {
	out := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		out[i] = c.Title
	}
	return out
}

// RatioSum returns the sum of Ratio across all columns of s.
func (s Set[T]) RatioSum() float64 {
	var sum float64
	for _, c := range s.Columns {
		sum += c.Ratio
	}
	return sum
}

// SelectColumns / Keys / Titles / RatioSum mirrors for GroupedSet.
func (g GroupedSet[T]) SelectColumns(keys []string) ([]GroupedColumn[T], error) {
	byKey := make(map[string]GroupedColumn[T], len(g.Columns))
	for _, c := range g.Columns {
		byKey[c.Key] = c
	}
	out := make([]GroupedColumn[T], 0, len(keys))
	var unknown []string
	for _, k := range keys {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		} else {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, &UnknownColumnError{Unknown: unknown, Valid: g.Keys()}
	}
	return out, nil
}

func (g GroupedSet[T]) Keys() []string {
	out := make([]string, len(g.Columns))
	for i, c := range g.Columns {
		out[i] = c.Key
	}
	return out
}

func (g GroupedSet[T]) Titles() []string {
	out := make([]string, len(g.Columns))
	for i, c := range g.Columns {
		out[i] = c.Title
	}
	return out
}

func (g GroupedSet[T]) RatioSum() float64 {
	var sum float64
	for _, c := range g.Columns {
		sum += c.Ratio
	}
	return sum
}

// UnknownColumnError is returned by SelectColumns when one or more
// requested keys are not present in the set.
type UnknownColumnError struct {
	Unknown []string
	Valid   []string
}

func (e *UnknownColumnError) Error() string {
	return "unknown column key(s): " + strings.Join(e.Unknown, ", ") +
		" (valid keys: " + strings.Join(e.Valid, ", ") + ")"
}
