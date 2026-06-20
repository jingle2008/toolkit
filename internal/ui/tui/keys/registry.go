package keys

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

/*
SortPrefix is the prefix used for sort key help text.
*/
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
	// BackHist is a key binding for navigating back in history.
	BackHist = key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("<[>", "History Back"),
	)
	// FwdHist is a key binding for navigating forward in history.
	FwdHist = key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("<]>", "History Forward"),
	)
	// ExportCSV is a key binding for exporting table data to CSV.
	ExportCSV = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("<e>", "Export CSV"),
	)
	// Delete is a key binding for deleting an item.
	Delete = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("<ctrl+x>", "Delete"),
	)
	// OpenPortal opens the selected tenant in the OCI console portal.
	// Bound to a control key so it never collides with typing into the
	// tenant-metadata form's Name/Note text fields.
	OpenPortal = key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("<ctrl+o>", "Open in portal"),
	)
	// Parent jumps from a sub-category back to its parent category,
	// re-selecting the parent row. Listed in each sub-category's catContext
	// (list view only); it shares the "o" key with CopyObject, which is
	// details-view only, so the two never coexist in the same view.
	Parent = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("<o>", "Parent"),
	)
	// OpenMetrics opens the selected DAC's OCI Telemetry MQL dashboard
	// in the browser. Bound to a plain letter (list view only) — no
	// text-entry view is active for the DAC list, so it cannot collide
	// with typing.
	OpenMetrics = key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("<m>", "Open Metrics"),
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
		key.WithKeys("r"),
		key.WithHelp("<r>", "Refresh"),
	)
	// RebootNode is a key binding for rebooting a GPU node.
	RebootNode = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("<shift+r>", "Reboot Node"),
	)
	// ToggleCordon is a key binding for cordoning a node in the GPU node list.
	ToggleCordon = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("<shift+c>", "Toggle Cordon"),
	)
	// DrainNode is a key binding for draining a node in the GPU node list.
	DrainNode = key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("<shift+d>", "Drain"),
	)
	// ScaleUp is a key binding for scaling up a GPU pool.
	ScaleUp = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("<shift+u>", "Scale Up"),
	)
	// SortInternal is a key binding for sorting by the "Internal" column.
	SortInternal = key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("<shift+i>", SortPrefix+common.InternalCol),
	)
	// SortValue is a key binding for sorting by the "Value" column.
	SortValue = key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("<shift+v>", SortPrefix+common.ValueCol),
	)
	// SortRegions is a key binding for sorting by the "Regions" column.
	SortRegions = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("<shift+r>", SortPrefix+common.RegionsCol),
	)
	// SortTenant is a key binding for sorting by the "Tenant" column.
	SortTenant = key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("<shift+t>", SortPrefix+common.TenantCol),
	)
	// SortContext is a key binding for sorting by the "Context" column.
	SortContext = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("<shift+c>", SortPrefix+common.ContextCol),
	)
	// SortSize is a key binding for sorting by the "Size" column.
	SortSize = key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("<shift+s>", SortPrefix+common.SizeCol),
	)
	// SortFree is a key binding for sorting by the "Free" column.
	SortFree = key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("<shift+f>", SortPrefix+common.FreeCol),
	)
	// SortAge is a key binding for sorting by the "Age" column.
	SortAge = key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("<shift+a>", SortPrefix+common.AgeCol),
	)
	// SortUsage is a key binding for sorting by the "Usage" column.
	SortUsage = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("<shift+u>", SortPrefix+common.UsageCol),
	)
	// ToggleAlias is a key binding for toggling the alias view.
	ToggleAlias = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("<ctrl+a>", "Toggle Alias"),
	)
	// SortType is a key binding for sorting by the "Type" column.
	SortType = key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("<shift+t>", SortPrefix+common.TypeCol),
	)
	// SortVendor is a key binding for sorting by the "Vendor" column.
	// Shares the V key with SortValue; the two never coexist in a single
	// category's binding set (SortValue lives on definition/override
	// categories, SortVendor is currently only on ImportedModel — the
	// one category still rendering a Vendor column).
	SortVendor = key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("<shift+v>", SortPrefix+common.VendorCol),
	)
	// ToggleFaulty is a key binding for toggling the faulty state.
	ToggleFaulty = key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("<ctrl+z>", "Toggle Faulty"),
	)
	// EditTenant opens the tenant-metadata entry form for an
	// unresolved DAC/ImportedModel row.
	EditTenant = key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("<shift+e>", "Edit Tenant"),
	)
)

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {SortSize, SortContext, ToggleFaulty, Refresh},
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
	domain.GPUPool: {
		common.ListView: {SortSize, ToggleFaulty, ScaleUp, Refresh},
	},
	domain.GPUNode: {
		common.ListView: {Parent, SortFree, SortType, SortAge, Refresh, ToggleCordon, DrainNode, ToggleFaulty, RebootNode, Delete},
	},
	domain.GPUWorkload: {
		common.ListView: {Parent, SortTenant, SortAge, OpenMetrics, ToggleFaulty, Refresh},
	},
	domain.DedicatedAICluster: {
		common.ListView: {Parent, SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, EditTenant, OpenMetrics, Refresh, ToggleFaulty, Delete},
	},
	domain.ImportedModel: {
		common.ListView: {Parent, SortTenant, SortSize, SortContext, SortVendor, CopyTenant, EditTenant, OpenMetrics, Refresh},
	},
	domain.LimitTenancyOverride: {
		common.ListView: {Parent, SortTenant, SortRegions, CopyTenant},
	},
	domain.ConsolePropertyTenancyOverride: {
		common.ListView: {Parent, SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.PropertyTenancyOverride: {
		common.ListView: {Parent, SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.LimitRegionalOverride: {
		common.ListView: {Parent, SortRegions},
	},
	domain.PropertyRegionalOverride: {
		common.ListView: {Parent, SortRegions, SortValue},
	},
	domain.ConsolePropertyRegionalOverride: {
		common.ListView: {Parent, SortRegions, SortValue},
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
