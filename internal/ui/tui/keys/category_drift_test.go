package keys

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
)

// noContextKeys are categories that intentionally have no per-category key
// bindings. A new category must be added here OR to catContext — never
// silently neither. Keep in sync with catContext (registry.go).
var noContextKeys = map[domain.Category]struct{}{
	domain.LimitDefinition: {},
	domain.ModelArtifact:   {},
	domain.Alias:           {},
}

/*
TestSortBindings_NameRealColumns ties each category's sort bindings to
its actual column titles.

A sort binding carries its target column in its help text
(SortPrefix + title); sortRows looks the title up in the rendered
headers and returns early when it finds nothing. So a binding aimed at
a column that doesn't exist — or one whose title was renamed out from
under it — compiles, registers, appears in the help line, and silently
does nothing when pressed. Nothing else catches that.
*/
func TestSortBindings_NameRealColumns(t *testing.T) {
	t.Parallel()
	for cat, modes := range catContext {
		if !columns.IsRegistered(cat) {
			continue
		}
		titles := make(map[string]struct{})
		for _, title := range columns.TitlesFor(cat) {
			titles[strings.ToLower(title)] = struct{}{}
		}
		for _, bindings := range modes {
			for _, b := range bindings {
				col, ok := strings.CutPrefix(b.Help().Desc, SortPrefix)
				if !ok {
					continue
				}
				_, found := titles[strings.ToLower(col)]
				assert.Truef(t, found,
					"%s binds %q but has no such column (titles: %v)",
					cat, col, columns.TitlesFor(cat))
			}
		}
	}
}

func TestCatContext_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		_, hasKeys := catContext[c]
		_, excluded := noContextKeys[c]
		assert.Truef(t, hasKeys != excluded,
			"%s must be in exactly one of catContext / noContextKeys (hasKeys=%v excluded=%v)",
			c, hasKeys, excluded)
	}
}
