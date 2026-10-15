package keys

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/view"
)

/*
TestHeaderHelp_FitsReferenceTerminal guards the header bar against
silently outgrowing a typical terminal as category actions are added.

The reference is a 120-column terminal with a 30-column info block, the
header logo, and two quick-switch slots. HeaderHelp may drop core keys to fit, but never a
category action: if this fails, the new binding needs a decision — trim
the category's catContext, or shorten a description — not a wider
reference.
*/
func TestHeaderHelp_FitsReferenceTerminal(t *testing.T) {
	t.Parallel()
	const (
		termWidth = 120
		infoWidth = 30
	)
	// mirrors updateLayout's budget
	width := termWidth - infoWidth - view.LogoWidth - view.LogoMargin - 1
	hm := help.New()
	measure := func(cols [][]key.Binding) int {
		return lipgloss.Width(hm.FullHelpView(cols))
	}
	qs := []key.Binding{
		key.NewBinding(key.WithKeys("0"), key.WithHelp("<0>", "dp-prod-ord ●")),
		key.NewBinding(key.WithKeys("1"), key.WithHelp("<1>", "dp-ppe-ord")),
	}
	for cat := range catContext {
		km := ResolveKeys(cat, common.ListView)
		km.QuickSwitch = qs
		cols := km.HeaderHelp(width, measure)
		descs := strings.Join(headerDescs(cols), ", ")
		for _, b := range km.Context {
			d := b.Help().Desc
			if strings.HasPrefix(d, SortPrefix) {
				continue
			}
			if !strings.Contains(descs, d) {
				t.Errorf("%s: action %q dropped from %d-column header: [%s]", cat, d, termWidth, descs)
			}
		}
	}
}
