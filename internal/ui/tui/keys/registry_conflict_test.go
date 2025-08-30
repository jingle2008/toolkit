package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestNoKeyConflictsPerCategoryAndMode(t *testing.T) {
	t.Parallel()

	modes := []common.ViewMode{common.ListView, common.DetailsView}

	for _, cat := range domain.Categories {
		for _, mode := range modes {
			km := ResolveKeys(cat, mode)
			seen := map[string]string{}

			check := func(setName string, bindings []key.Binding) {
				for _, b := range bindings {
					for _, k := range b.Keys() {
						if prev, ok := seen[k]; ok {
							t.Errorf("duplicate key %q for category %s mode %v: previously in %s; now in %s (%s)", k, cat, mode, prev, setName, b.Help().Desc)
						} else {
							desc := b.Help().Desc
							if desc == "" {
								desc = "(no-desc)"
							}
							seen[k] = setName + ":" + desc
						}
					}
				}
			}

			check("Global", km.Global)
			check("Mode", km.Mode)
			check("Context", km.Context)
		}
	}
}
