package keys

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
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

func headerDescs(cols [][]key.Binding) []string {
	var out []string
	for _, col := range cols {
		for _, b := range col {
			out = append(out, b.Help().Desc)
		}
	}
	return out
}

func TestHeaderHelp_RanksActionsThenCoreThenHelp(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global:  []key.Binding{Help, Quit, Back, ToggleLog},
		Mode:    []key.Binding{FilterMode, NextCategory},
		Context: []key.Binding{Refresh, SortAge, Delete},
	}
	got := headerDescs(km.HeaderHelp(0, nil))
	want := []string{"Delete", "Refresh", "Back/Clear", "Filter mode", "Help"}
	if !slices.Equal(got, want) {
		t.Errorf("HeaderHelp() = %v, want %v", got, want)
	}
}

func TestHeaderHelp_DropsCoreBeforeActionsAndCountsHidden(t *testing.T) {
	t.Parallel()
	km := KeyMap{
		Global:  []key.Binding{Help, Back},
		Mode:    []key.Binding{FilterMode, CommandMode},
		Context: []key.Binding{Refresh, Delete},
	}
	// Budget for exactly four bindings, Help included.
	count := func(cols [][]key.Binding) int { return len(headerDescs(cols)) }
	got := headerDescs(km.HeaderHelp(4, count))
	want := []string{"Delete", "Refresh", "Back/Clear", "Help +2 more"}
	if !slices.Equal(got, want) {
		t.Errorf("HeaderHelp() = %v, want %v", got, want)
	}
}

func TestHeaderHelp_QuickSwitchLeads(t *testing.T) {
	t.Parallel()
	qs := key.NewBinding(key.WithKeys("0"), key.WithHelp("<0>", "dp-dev-phx"))
	km := KeyMap{Global: []key.Binding{Help}, QuickSwitch: []key.Binding{qs}}
	cols := km.HeaderHelp(0, nil)
	if len(cols) != 2 || cols[0][0].Help().Desc != "dp-dev-phx" {
		t.Errorf("quick-switch column not leading: %v", headerDescs(cols))
	}
}

func TestHeaderHelp_NothingWhenHelpAloneOverflows(t *testing.T) {
	t.Parallel()
	km := KeyMap{Global: []key.Binding{Help}, Context: []key.Binding{Refresh}}
	count := func(cols [][]key.Binding) int { return len(headerDescs(cols)) }
	if got := km.HeaderHelp(0, nil); len(got) == 0 {
		t.Fatal("width 0 must disable fitting")
	}
	if got := km.HeaderHelp(-1, nil); len(got) == 0 {
		t.Fatal("negative width must disable fitting")
	}
	if got := km.HeaderHelp(1, func(c [][]key.Binding) int { return count(c) + 1 }); got != nil {
		t.Errorf("HeaderHelp() = %v, want nil", headerDescs(got))
	}
}
