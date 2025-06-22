package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestShortHelpAndFullHelp(t *testing.T) {
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

func TestHelpAndQuit(t *testing.T) {
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
