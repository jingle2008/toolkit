package keys

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/domain"
)

// keyMap holds key bindings for the toolkit UI.
type KeyMap struct {
	Help               key.Binding
	Quit               key.Binding
	NextCategory       key.Binding
	PrevCategory       key.Binding
	FilterItems        key.Binding
	JumpTo             key.Binding
	ViewDetails        key.Binding
	ApplyContext       key.Binding
	ViewModelArtifacts key.Binding
	Category           domain.Category
	Additionals        map[domain.Category][]key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		k.Additionals[k.Category],
		{k.NextCategory, k.PrevCategory, k.FilterItems, k.JumpTo}, // first column
		{k.ViewDetails, k.ApplyContext, k.Help, k.Quit},           // second column
	}
}

var viewModelArtifacts = key.NewBinding(
	key.WithKeys("a"),
	key.WithHelp("a", "view artifacts"),
)

var Keys = KeyMap{
	NextCategory: key.NewBinding(
		key.WithKeys("shift+right"),
		key.WithHelp("shift+→", "next category"),
	),
	PrevCategory: key.NewBinding(
		key.WithKeys("shift+left"),
		key.WithHelp("shift+←", "previous category"),
	),
	FilterItems: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter items"),
	),
	JumpTo: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "jump to category"),
	),
	ViewDetails: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "view details"),
	),
	ApplyContext: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply context"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	ViewModelArtifacts: viewModelArtifacts,
	Additionals: map[domain.Category][]key.Binding{
		domain.BaseModel: {viewModelArtifacts},
	},
}
