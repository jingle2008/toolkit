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
//
// RenderForExport is an optional alternate renderer used by file/CSV
// export paths (TUI <e> and CLI `-o csv`/`-o tsv` when env is set).
// Use it when the export-appropriate value is fundamentally different
// from the display value — e.g., expanding an OCID-suffix Name into
// the fully-qualified ocid1.<type>.<realm>.<region>.<suffix> form
// that downstream OCI tooling expects. Nil means "use Render".
//
// The signature carries both `realm` and `region` even though most
// flat categories won't reference either — keeps the export-mode
// contract symmetric with GroupedColumn.RenderForExport and leaves
// room for future flat columns whose export form depends on env
// (e.g. a tenancy OCID column on the Tenant view).
type Column[T any] struct {
	Title          string
	Key            string
	Ratio          float64
	Render         func(T) string
	RenderForExport   func(realm, region string, item T) string
	TruncateMiddle bool
}

// GroupedColumn is a column for a grouped category (loader returns
// map[string][]T). Render receives both the group key and the item;
// any column can use either. A "group key column" is just a
// GroupedColumn whose Render ignores `item` and returns `key`.
//
// TruncateMiddle and RenderForExport have the same semantics as
// Column.TruncateMiddle / Column.RenderForExport; RenderForExport's
// signature carries the group key alongside realm/region so a
// column can substitute its display value with an export-mode
// representation that depends on either.
type GroupedColumn[T any] struct {
	Title          string
	Key            string
	Ratio          float64
	Render         func(key string, item T) string
	RenderForExport   func(realm, region, key string, item T) string
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

// selectByKey returns the subset of cols whose key (extracted via
// keyOf) appears in wanted, preserving the order of wanted. Any
// keys in wanted that aren't present in cols are bundled into a
// single UnknownKeyError so the CLI can show one complete message.
// validKeys is a thunk so the keys slice is only materialized on the
// unknown-key error path; the success path skips the allocation.
func selectByKey[T any](cols []T, keyOf func(T) string, wanted []string, validKeys func() []string) ([]T, error) {
	byKey := make(map[string]T, len(cols))
	for _, c := range cols {
		byKey[keyOf(c)] = c
	}
	out := make([]T, 0, len(wanted))
	var unknown []string
	for _, k := range wanted {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		} else {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, &UnknownKeyError{Unknown: unknown, Valid: validKeys()}
	}
	return out, nil
}

// mapColumns projects each col through extract, preserving order.
func mapColumns[T any, F any](cols []T, extract func(T) F) []F {
	out := make([]F, len(cols))
	for i, c := range cols {
		out[i] = extract(c)
	}
	return out
}

// sumColumns sums extract(col) across cols.
func sumColumns[T any](cols []T, extract func(T) float64) float64 {
	var sum float64
	for _, c := range cols {
		sum += extract(c)
	}
	return sum
}

// Select returns the columns of s whose Key is in keys, in the order
// given by keys. Returns an error listing all unknown keys (so the
// CLI can show a single complete message).
func (s Set[T]) Select(keys []string) ([]Column[T], error) {
	return selectByKey(s.Columns, func(c Column[T]) string { return c.Key }, keys, s.Keys)
}

// Keys returns the keys declared on s in order.
func (s Set[T]) Keys() []string {
	return mapColumns(s.Columns, func(c Column[T]) string { return c.Key })
}

// Titles returns the Title for each column of s in declared order.
func (s Set[T]) Titles() []string {
	return mapColumns(s.Columns, func(c Column[T]) string { return c.Title })
}

// RatioSum returns the sum of Ratio across all columns of s.
func (s Set[T]) RatioSum() float64 {
	return sumColumns(s.Columns, func(c Column[T]) float64 { return c.Ratio })
}

// Select / Keys / Titles / RatioSum mirrors for GroupedSet.
func (g GroupedSet[T]) Select(keys []string) ([]GroupedColumn[T], error) {
	return selectByKey(g.Columns, func(c GroupedColumn[T]) string { return c.Key }, keys, g.Keys)
}

// Keys returns the keys declared on g in order.
func (g GroupedSet[T]) Keys() []string {
	return mapColumns(g.Columns, func(c GroupedColumn[T]) string { return c.Key })
}

// Titles returns the column titles declared on g in order.
func (g GroupedSet[T]) Titles() []string {
	return mapColumns(g.Columns, func(c GroupedColumn[T]) string { return c.Title })
}

// RatioSum returns the sum of column Ratio values on g.
func (g GroupedSet[T]) RatioSum() float64 {
	return sumColumns(g.Columns, func(c GroupedColumn[T]) float64 { return c.Ratio })
}

// UnknownKeyError is returned by Set.Select / GroupedSet.Select when
// one or more requested keys are not present in the set.
type UnknownKeyError struct {
	Unknown []string
	Valid   []string
}

func (e *UnknownKeyError) Error() string {
	return "unknown column key(s): " + strings.Join(e.Unknown, ", ") +
		" (valid keys: " + strings.Join(e.Valid, ", ") + ")"
}
