package keys

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// noContextKeys are categories that intentionally have no per-category key
// bindings. A new category must be added here OR to catContext — never
// silently neither. Keep in sync with catContext (registry.go).
var noContextKeys = map[domain.Category]struct{}{
	domain.ModelArtifact: {},
	domain.Alias:         {},
}

/*
TestSortBindings_NameRealColumns ties each category's sort bindings to
its actual column titles.

A sort binding carries its target column in its help text
(sortPrefix + title); sortRows looks the title up in the rendered
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
				col, ok := SortColumnOf(b)
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
	for _, c := range domain.Categories() {
		_, hasKeys := catContext[c]
		_, excluded := noContextKeys[c]
		assert.Truef(t, hasKeys != excluded,
			"%s must be in exactly one of catContext / noContextKeys (hasKeys=%v excluded=%v)",
			c, hasKeys, excluded)
	}
}

// Ctrl+Z is bound on exactly the categories whose rows can be faulty, the
// set `toolkit get --faulty` and MCP's faulty input accept too.
func TestToggleFaulty_BoundOnFaultyCategories(t *testing.T) {
	t.Parallel()
	var bound []domain.Category
	for c, byMode := range catContext {
		for _, b := range byMode[common.ListView] {
			if b.Help() == ToggleFaulty.Help() {
				bound = append(bound, c)
			}
		}
	}
	assert.ElementsMatch(t, relations.FaultyCategories(), bound)
}

// Every category lists its actions in actionOrder — the row's own first,
// faulty-only and refresh last — so the footer reads the same everywhere
// and drops the list toggles before any row action.
func TestCatContext_ActionOrder(t *testing.T) {
	t.Parallel()
	rank := func(b key.Binding) int {
		for i, o := range actionOrder {
			if slices.Equal(b.Keys(), o.Keys()) {
				return i
			}
		}
		return -1
	}
	for cat, byMode := range catContext {
		last := -1
		for _, b := range byMode[common.ListView] {
			if isSort(b) {
				continue
			}
			r := rank(b)
			assert.GreaterOrEqualf(t, r, 0, "%s: %q is missing from actionOrder", cat, b.Help().Desc)
			assert.Greaterf(t, r, last, "%s: %q is out of order", cat, b.Help().Desc)
			last = r
		}
	}
}

// GPUNode names ^x Terminate: deleting a node terminates its instance.
func TestGPUNode_DeleteIsTerminate(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.GPUNode, common.ListView)
	assert.True(t, slices.ContainsFunc(km.Context, func(b key.Binding) bool { return b.Help().Desc == "Terminate" }))
	assert.False(t, slices.ContainsFunc(km.Context, func(b key.Binding) bool { return b.Help().Desc == "Delete" }))
}
