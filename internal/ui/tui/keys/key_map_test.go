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
