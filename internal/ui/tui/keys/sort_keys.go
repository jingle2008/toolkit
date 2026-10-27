package keys

import (
	"slices"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// SortLeader is the prefix key for sorting: SortLeader then a column's
// key sorts by that column.
const SortLeader = "s"

// sortBy declares a sortable column. It carries no key of its own:
// ResolveKeys gives it the category's SortLeader sequence (see SortKeys).
func sortBy(column string) key.Binding {
	return key.NewBinding(key.WithHelp("", sortPrefix+column))
}

// sortColumns lists the columns cat's list view can sort by, in binding
// order: Name (every list view) first, then the category's own.
func sortColumns(cat domain.Category) []string {
	var cols []string
	for _, b := range slices.Concat(listModeKeys, catContext[cat][common.ListView]) {
		if col, ok := SortColumnOf(b); ok {
			cols = append(cols, col)
		}
	}
	return cols
}

// SortKeys maps each column cat's list view can sort by to its sort key,
// a lowercase letter typed after SortLeader. See deriveSortKeys.
func SortKeys(cat domain.Category) map[string]string {
	return deriveSortKeys(sortColumns(cat))
}

// pinnedSortKeys fixes the key of columns whose first letter would
// otherwise depend on which other columns a category sorts by: s is always
// Status and z always siZe (DedicatedAICluster sorts by Size, Tenant, Age
// and Usage, which leave no letter of "status" free), and x is maX (its
// first free letter would be a, which means Age elsewhere).
var pinnedSortKeys = map[string]rune{
	common.StatusCol: 's',
	common.SizeCol:   'z',
	common.MaxCol:    'x',
}

// deriveSortKeys gives a pinned column (pinnedSortKeys) its pinned key, and
// every other column the first letter of its name or, when an earlier or
// pinned column already took that letter, the next letter of its name
// still free. Columns are taken in order, so Name always gets "n". A
// column with no free letter gets no key.
func deriveSortKeys(cols []string) map[string]string {
	used := map[rune]bool{}
	out := make(map[string]string, len(cols))
	for _, col := range cols {
		if r, ok := pinnedSortKeys[col]; ok {
			used[r] = true
			out[col] = string(r)
		}
	}
	for _, col := range cols {
		if _, ok := out[col]; ok {
			continue
		}
		for _, r := range strings.ToLower(col) {
			if unicode.IsLetter(r) && !used[r] {
				used[r] = true
				out[col] = string(r)
				break
			}
		}
	}
	return out
}

// SortColumn returns the column cat sorts by for sort key k.
func SortColumn(cat domain.Category, k string) (string, bool) {
	for col, ck := range SortKeys(cat) {
		if ck == k {
			return col, true
		}
	}
	return "", false
}

// withSortKeys returns bs with every sort declaration bound to its
// SortLeader sequence ("s n"), which help shows as the key. The sequence
// never matches a single keypress; the model resolves it.
func withSortKeys(bs []key.Binding, sortKeys map[string]string) []key.Binding {
	out := slices.Clone(bs)
	for i, b := range out {
		col, ok := SortColumnOf(b)
		if !ok {
			continue
		}
		if k, ok := sortKeys[col]; ok {
			seq := SortLeader + " " + k
			out[i] = key.NewBinding(key.WithKeys(seq), key.WithHelp(seq, b.Help().Desc))
		}
	}
	return out
}
