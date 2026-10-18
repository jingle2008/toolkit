package keys

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestShortHelpAndFullHelp(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global: []key.Binding{
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "help")),
		},
		Mode: []key.Binding{
			key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
		},
		Context: []key.Binding{
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "context")),
		},
	}
	short := km.ShortHelp()
	if len(short) != 2 {
		t.Errorf("ShortHelp() = %d, want 2", len(short))
	}
	full := km.FullHelp()
	if len(full) == 0 {
		t.Error("FullHelp() returned empty")
	}
}

func TestSortableColumns(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Mode: []key.Binding{
			key.NewBinding(key.WithKeys("N"), key.WithHelp("<shift+n>", SortPrefix+"Name")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("<c>", "Copy")),
		},
		Context: []key.Binding{
			key.NewBinding(key.WithKeys("V"), key.WithHelp("<shift+v>", SortPrefix+"Vendor")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("<r>", "Refresh")),
		},
	}
	got := km.SortableColumns()
	if !got["name"] || !got["vendor"] {
		t.Errorf("expected name and vendor (lowercased) in sortable set, got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected exactly 2 sortable columns, got %v", got)
	}
}

func TestHelpAndQuit(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global: []key.Binding{
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
			key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "help")),
		},
	}
	if km.Help().Help().Key != "h" {
		t.Error("Help() did not return help key")
	}
	if km.Quit().Help().Key != "q" {
		t.Error("Quit() did not return quit key")
	}
}

func TestFullHelp_QuickSwitchLeadsInSlotOrder(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global: []key.Binding{
			key.NewBinding(key.WithKeys("a"), key.WithHelp("<a>", "Alpha")),
		},
		QuickSwitch: []key.Binding{
			key.NewBinding(key.WithKeys("0"), key.WithHelp("<0>", "dp-dev-phx")),
			key.NewBinding(key.WithKeys("2"), key.WithHelp("<2>", "dp-dev-iad")),
		},
	}
	full := km.FullHelp()
	if len(full) != 2 {
		t.Fatalf("FullHelp() = %d columns, want 2", len(full))
	}
	if got := full[0][1].Help().Desc; got != "dp-dev-iad" {
		t.Errorf("quick-switch column reordered: second entry = %q", got)
	}
	if got := full[1][0].Help().Desc; got != "Alpha" {
		t.Errorf("second column = %q, want Alpha", got)
	}
}

func TestFooterBindings_ActionsThenSortThenCore(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global:  []key.Binding{Help, Quit, Back, ToggleLog},
		Mode:    []key.Binding{FilterMode, NextCategory, SortName},
		Context: []key.Binding{Refresh, SortAge, Delete},
	}
	actions, core := km.FooterBindings()
	descs := func(bs []key.Binding) []string {
		out := make([]string, 0, len(bs))
		for _, b := range bs {
			out = append(out, b.Help().Desc)
		}
		return out
	}
	if got, want := descs(actions), []string{"Refresh", "Delete", "sort"}; !slices.Equal(got, want) {
		t.Errorf("FooterBindings() actions = %v, want %v", got, want)
	}
	if got, want := descs(core), []string{"Filter"}; !slices.Equal(got, want) {
		t.Errorf("FooterBindings() core = %v, want %v", got, want)
	}
}

func TestFooterBindings_NoSortHintWithoutSorts(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.Tenant, common.DetailsView)
	actions, _ := km.FooterBindings()
	for _, b := range actions {
		if b.Help().Desc == "sort" {
			t.Errorf("details view offers sort: %v", b.Help())
		}
	}
}
