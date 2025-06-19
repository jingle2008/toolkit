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
		key.WithHelp("esc", "go back"),
	)
	CopyName = key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "copy name"),
	)
)

var globalKeys = []key.Binding{
	CopyName,
	Back,
	Help,
	Quit,
}

// GlobalKeys returns a copy of the global key bindings (always active).
func GlobalKeys() []key.Binding { return globalKeys }

// ListModeKeys returns a copy of the key bindings for list mode.
func ListModeKeys() []key.Binding { return listModeKeys }

// DetailsModeKeys returns a copy of the key bindings for details mode.
func DetailsModeKeys() []key.Binding { return detailsModeKeys }

// CatContext returns the mapping of category and view mode to context-specific key bindings.
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
	Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply context"),
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

var (
	// ViewModelArtifacts is a key binding for viewing artifacts in the base model list view.
	ViewModelArtifacts = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "view artifacts"),
	)
	// CopyValue is a key binding for copying the value of an item in the details view.
	CopyValue = key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "copy value"),
	)
	// CopyTenant is a key binding for copying the tenant ID in the tenant context.
	CopyTenant = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "copy tenant"),
	)
)

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {ViewModelArtifacts},
	},
	domain.Tenant: {
		common.ListView: {Confirm, CopyTenant},
	},
	domain.LimitDefinition: {
		common.ListView: {Confirm},
	},
	domain.ConsolePropertyDefinition: {
		common.ListView: {Confirm},
	},
	domain.PropertyDefinition: {
		common.ListView: {Confirm},
	},
	domain.GpuPool: {
		common.ListView: {Confirm},
	},
	domain.DedicatedAICluster: {
		common.ListView: {CopyTenant, CopyValue},
	},
	domain.LimitTenancyOverride: {
		common.ListView: {CopyTenant},
	},
	domain.ConsolePropertyTenancyOverride: {
		common.ListView: {CopyTenant},
	},
	domain.PropertyTenancyOverride: {
		common.ListView: {CopyTenant, CopyValue},
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
