package keys

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// Global key bindings (always active)
var (
	Quit = key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	)
	Help = key.NewBinding(
		key.WithKeys("?", "h"),
		key.WithHelp("?/h", "toggle help"),
	)
	Back = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back to last state"),
	)
)

var globalKeys = []key.Binding{
	Back,
	Help,
	Quit,
}

// Exported helpers for full help aggregation
func GlobalKeys() []key.Binding      { return append([]key.Binding(nil), globalKeys...) }
func ListModeKeys() []key.Binding    { return append([]key.Binding(nil), listModeKeys...) }
func DetailsModeKeys() []key.Binding { return append([]key.Binding(nil), detailsModeKeys...) }
func CatContext() map[domain.Category]map[common.ViewMode][]key.Binding {
	return catContext
}

// FullKeyMap returns a KeyMap with all unique keys in each section.
func FullKeyMap() KeyMap {
	// Context keys
	ctx := []key.Binding{}
	seen := map[string]struct{}{}
	add := func(b key.Binding) {
		k := b.Help().Key + "|" + b.Help().Desc
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			ctx = append(ctx, b)
		}
	}

	for _, byMode := range catContext {
		for _, bindings := range byMode {
			for _, b := range bindings {
				add(b)
			}
		}
	}

	return KeyMap{
		Global:  GlobalKeys(),
		Mode:    append(listModeKeys, detailsModeKeys...),
		Context: ctx,
	}
}

// Mode-specific key bindings
var (
	NextCategory = key.NewBinding(
		key.WithKeys("shift+right"),
		key.WithHelp("shift+→", "next category"),
	)
	PrevCategory = key.NewBinding(
		key.WithKeys("shift+left"),
		key.WithHelp("shift+←", "previous category"),
	)
	FilterItems = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter items"),
	)
	JumpTo = key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "jump to category"),
	)
	ViewDetails = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "view details"),
	)
	Apply = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply context"),
	)
	ViewModelArtifacts = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "view artifacts"),
	)
)

var listModeKeys = []key.Binding{
	NextCategory,
	PrevCategory,
	FilterItems,
	JumpTo,
	ViewDetails,
}

var detailsModeKeys = []key.Binding{}

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {ViewModelArtifacts},
	},
	domain.Tenant: {
		common.ListView: {Apply},
	},
	domain.LimitDefinition: {
		common.ListView: {Apply},
	},
	domain.ConsolePropertyDefinition: {
		common.ListView: {Apply},
	},
	domain.PropertyDefinition: {
		common.ListView: {Apply},
	},
	domain.GpuPool: {
		common.ListView: {Apply},
	},
}

// ResolveKeys returns the composed KeyMap for the given category and UI mode.
func ResolveKeys(cat domain.Category, mode common.ViewMode) KeyMap {
	km := KeyMap{
		Global: globalKeys,
	}
	switch mode {
	case common.ListView:
		km.Mode = listModeKeys
	case common.DetailsView:
		km.Mode = detailsModeKeys
	}
	if ctxByMode, ok := catContext[cat]; ok {
		km.Context = ctxByMode[mode]
	}
	return km
}
