package tui

import (
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// TestSortBindings_MatchColumnTitles guards the stringly-typed bond
// between keys.Sort* bindings and column titles. The ↕ indicator and
// the sortTableByColumn dispatch both rely on the binding's help
// description (after stripping "Sort ") equaling a real column title
// in the active category. A drift in either direction — a Sort*
// binding pointing at a column that no longer exists, or a renamed
// column whose Sort* binding still uses the old title — would
// silently disable sorting for that column, with no compile-time or
// runtime error.
func TestSortBindings_MatchColumnTitles(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		cat := cat
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()
			km := keys.ResolveKeys(cat, common.ListView)
			sortable := km.SortableColumns()
			if len(sortable) == 0 {
				return // not every category has sort bindings, that's fine
			}
			headers := getHeaders(cat)
			titles := map[string]bool{}
			for _, h := range headers {
				titles[strings.ToLower(h.text)] = true
			}
			for col := range sortable {
				if !titles[col] {
					t.Errorf("category %s: Sort* binding for %q has no matching column title; available: %v",
						cat, col, headerTitles(headers))
				}
			}
		})
	}
}

func headerTitles(hs []header) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.text
	}
	return out
}
