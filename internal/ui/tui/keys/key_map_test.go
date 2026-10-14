package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
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
