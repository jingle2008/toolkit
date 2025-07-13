package keys

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

const SortPrefix = "Sort "

// Global key bindings (always active)
var (
	Quit = key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("<q>", "Quit"),
	)
	Help = key.NewBinding(
		key.WithKeys("?", "h"),
		key.WithHelp("<?/h>", "Help"),
	)
	Back = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("<esc>", "Back/Clear"),
	)
	ViewDetails = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("<y>", "Toggle Details"),
	)
	CopyName = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("<c>", "Copy"),
	)
)

var globalKeys = []key.Binding{
	Help,
	ViewDetails,
	CopyName,
	Back,
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
		key.WithKeys("tab"),
		key.WithHelp("<tab>", "Next Category"),
	)
	PrevCategory = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("<shift+tab>", "Previous Category"),
	)
	FilterMode = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("</term>", "Filter mode"),
	)
	CommandMode = key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp("<:cmd>", "Command mode"),
	)
	PasteFilter = key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("<p>", "Paste Filter"),
	)
	Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("<enter>", "View/Enter"),
	)
	SortName = key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("<shift+n>", SortPrefix+"Name"),
	)
)

var (
	BackHist = key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("<[>", "History Back"),
	)
	FwdHist = key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("<]>", "History Forward"),
	)
)

var listModeKeys = []key.Binding{
	BackHist,
	FwdHist,
	NextCategory,
	PrevCategory,
	CommandMode,
	FilterMode,
	PasteFilter,
	SortName,
	Confirm,
	ShowAlias,
}

var detailsModeKeys = []key.Binding{
	CopyObject,
}

var (
	// CopyObject is a key binding for copying the value of an item in the details view.
	CopyObject = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("<o>", "Copy Object"),
	)
	// CopyTenant is a key binding for copying the tenant ID in the tenant context.
	CopyTenant = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("<t>", "Copy Tenant ID"),
	)
	// Refresh is a key binding for refreshing the current view or data.
	Refresh = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("<ctrl+r>", "Refresh"),
	)
	// CordonNode is a key binding for cordoning a node in the GPU node list.
	CordonNode = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("<c>", "Cordon"),
	)
	UncordonNode = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("<u>", "Uncordon"),
	)
	// DrainNode is a key binding for draining a node in the GPU node list.
	DrainNode = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("<r>", "Drain"),
	)
	SortInternal = key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("<shift+i>", SortPrefix+"Internal"),
	)
	SortValue = key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("<shift+v>", SortPrefix+"Value"),
	)
	SortRegions = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("<shift+r>", SortPrefix+"Regions"),
	)
	SortTenant = key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("<shift+t>", SortPrefix+"Tenant"),
	)
	SortMaxTokens = key.NewBinding(
		key.WithKeys("M"),
		key.WithHelp("<shift+m>", SortPrefix+"Max Tokens"),
	)
	SortSize = key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("<shift+s>", SortPrefix+"Size"),
	)
	SortFree = key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("<shift+f>", SortPrefix+"Free"),
	)
	SortAge = key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("<shift+a>", SortPrefix+"Age"),
	)
	SortUsage = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("<shift+u>", SortPrefix+"Usage"),
	)
	ShowAlias = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("<ctrl+a>", "Show Alias"),
	)
)

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {SortMaxTokens},
	},
	domain.Tenant: {
		common.ListView: {SortInternal, CopyTenant},
	},

	domain.ConsolePropertyDefinition: {
		common.ListView: {SortValue},
	},
	domain.PropertyDefinition: {
		common.ListView: {SortValue},
	},
	domain.GpuPool: {
		common.ListView: {SortSize},
	},
	domain.GpuNode: {
		common.ListView: {SortFree, SortAge, Refresh, CordonNode, DrainNode, UncordonNode},
	},
	domain.DedicatedAICluster: {
		common.ListView: {SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, Refresh},
	},
	domain.LimitTenancyOverride: {
		common.ListView: {SortTenant, SortRegions, CopyTenant},
	},
	domain.ConsolePropertyTenancyOverride: {
		common.ListView: {SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.PropertyTenancyOverride: {
		common.ListView: {SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.LimitRegionalOverride: {
		common.ListView: {SortRegions},
	},
	domain.PropertyRegionalOverride: {
		common.ListView: {SortRegions, SortValue},
	},
	domain.ConsolePropertyRegionalOverride: {
		common.ListView: {SortRegions, SortValue},
	},
}

// ResolveKeys returns the composed KeyMap for the given category and UI mode.
func ResolveKeys(cat domain.Category, mode common.ViewMode) KeyMap {
	ViewDetails.SetEnabled(cat != domain.Alias) // no details to view
	CopyName.SetEnabled(cat != domain.GpuNode)  // conflict with cordon
	for i, b := range globalKeys {
		if b.Help() == ViewDetails.Help() {
			globalKeys[i].SetEnabled(cat != domain.Alias)
		} else if b.Help() == CopyName.Help() {
			globalKeys[i].SetEnabled(cat != domain.GpuNode)
		}
	}

	km := KeyMap{
		Global: globalKeys,
	}
	switch mode { //nolint:exhaustive
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
