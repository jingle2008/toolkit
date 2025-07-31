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
		key.WithHelp("<shift+n>", SortPrefix+common.NameCol),
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
	ExportCSV = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("<e>", "Export CSV"),
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
	ToggleAlias,
	ExportCSV,
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
	// ToggleCordon is a key binding for cordoning a node in the GPU node list.
	ToggleCordon = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("<shift+c>", "Toggle Cordon"),
	)
	// DrainNode is a key binding for draining a node in the GPU node list.
	DrainNode = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("<r>", "Drain"),
	)
	SortInternal = key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("<shift+i>", SortPrefix+common.InternalCol),
	)
	SortValue = key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("<shift+v>", SortPrefix+common.ValueCol),
	)
	SortRegions = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("<shift+r>", SortPrefix+common.RegionsCol),
	)
	SortTenant = key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("<shift+t>", SortPrefix+common.TenantCol),
	)
	SortMaxTokens = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("<shift+c>", SortPrefix+common.ContextCol),
	)
	SortSize = key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("<shift+s>", SortPrefix+common.SizeCol),
	)
	SortFree = key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("<shift+f>", SortPrefix+common.FreeCol),
	)
	SortAge = key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("<shift+a>", SortPrefix+common.AgeCol),
	)
	SortUsage = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("<shift+u>", SortPrefix+common.UsageCol),
	)
	ToggleAlias = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("<ctrl+a>", "Toggle Alias"),
	)
	SortType = key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("<shift+t>", SortPrefix+common.TypeCol),
	)
	ToggleFaulty = key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("<ctrl+z>", "Toggle Faulty"),
	)
)

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {SortSize, SortMaxTokens, ToggleFaulty},
	},
	domain.Tenant: {
		common.ListView: {SortInternal, CopyTenant, ToggleFaulty},
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
		common.ListView: {SortFree, SortType, SortAge, Refresh, ToggleCordon, DrainNode, ToggleFaulty},
	},
	domain.DedicatedAICluster: {
		common.ListView: {SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, Refresh, ToggleFaulty},
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
	domain.Environment: {
		common.ListView: {SortType},
	},
	domain.ServiceTenancy: {
		common.ListView: {SortType},
	},
}

// ResolveKeys returns the composed KeyMap for the given category and UI mode.
func ResolveKeys(cat domain.Category, mode common.ViewMode) KeyMap {
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
