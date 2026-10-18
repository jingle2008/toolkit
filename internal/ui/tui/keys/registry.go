package keys

import (
	"charm.land/bubbles/v2/key"

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
		key.WithHelp("q", "Quit"),
	)
	Help = key.NewBinding(
		key.WithKeys("?", "h"),
		key.WithHelp("?/h", "Help"),
	)
	Back = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "Back/Clear"),
	)
	ViewDetails = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "Toggle details"),
	)
	CopyName = key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "Copy"),
	)
	ToggleLog = key.NewBinding(
		key.WithKeys("`"),
		key.WithHelp("`", "Toggle log"),
	)
)

var globalKeys = []key.Binding{
	Help,
	ViewDetails,
	CopyName,
	ToggleLog,
	Back,
	Quit,
}

// Mode-specific key bindings
var (
	NextCategory = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "Next category"),
	)
	PrevCategory = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "Previous category"),
	)
	FilterMode = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "Filter"),
	)
	CommandMode = key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "Command palette"),
	)
	PasteFilter = key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "Paste filter"),
	)
	Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "Open"),
	)
	SortName = sortBy(common.NameCol)
)

var (
	// QuickSwitch matches the recent-context slot keys; the slot is the
	// pressed digit. Per-slot help entries are built by the model.
	QuickSwitch = key.NewBinding(
		key.WithKeys("0", "1", "2", "3", "4", "5"),
	)
	// BackHist is a key binding for navigating back in history.
	BackHist = key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "History back"),
	)
	// FwdHist is a key binding for navigating forward in history.
	FwdHist = key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "History forward"),
	)
	// ExportCSV is a key binding for exporting table data to CSV.
	ExportCSV = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "Export CSV"),
	)
	// Delete is a key binding for deleting an item.
	Delete = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("^x", "Delete"),
	)
	// OpenPortal opens the selected tenant in the OCI console portal.
	// Bound to a control key so it never collides with typing into the
	// tenant-metadata form's Name/Note text fields.
	OpenPortal = key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("^o", "Open in DevOps portal"),
	)
	// GoTo opens a picker of the rows the selected row relates to (its
	// DAC, model, runtime, tenant, ...) and jumps to the chosen one. Not
	// "g": the table already binds that to go-to-top, and both would fire.
	// It shares "o" with CopyObject, which is details-view only, so the
	// two never coexist in the same view.
	GoTo = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "Go to related"),
	)
	// OpenMetrics opens the selected DAC's OCI Telemetry MQL dashboard
	// in the browser. Bound to a plain letter (list view only) — no
	// text-entry view is active for the DAC list, so it cannot collide
	// with typing.
	OpenMetrics = key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "Open metrics"),
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
	TogglePreview,
	ExportCSV,
}

var detailsModeKeys = []key.Binding{
	CopyObject,
	ToggleJSON,
}

var (
	// ToggleJSON switches the details view between its fields layout and
	// the item's raw JSON.
	ToggleJSON = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "Fields/JSON"),
	)
	// CopyObject is a key binding for copying the value of an item in the details view.
	CopyObject = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "Copy object"),
	)
	// CopyTenant is a key binding for copying the tenant ID in the tenant context.
	CopyTenant = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "Copy tenant ID"),
	)
	// Refresh is a key binding for refreshing the current view or data.
	Refresh = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "Refresh"),
	)
	// Restart is a key binding for restarting the selected row: a GPU
	// node's instance (soft reset) or an InferenceService (rolling restart).
	Restart = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "Restart"),
	)
	// ToggleCordon is a key binding for cordoning a node in the GPU node list.
	ToggleCordon = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "Toggle cordon"),
	)
	// DrainNode is a key binding for draining a node in the GPU node list.
	DrainNode = key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "Drain"),
	)
	// ScaleUp is a key binding for scaling up a GPU pool.
	ScaleUp = key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "Scale up"),
	)
	// SortInternal is a key binding for sorting by the "Internal" column.
	SortInternal = sortBy(common.InternalCol)
	// SortValue is a key binding for sorting by the "Value" column.
	SortValue = sortBy(common.ValueCol)
	// SortRegions is a key binding for sorting by the "Regions" column.
	SortRegions = sortBy(common.RegionsCol)
	// SortTenant is a key binding for sorting by the "Tenant" column.
	SortTenant = sortBy(common.TenantCol)
	// SortContext is a key binding for sorting by the "Context" column.
	SortContext = sortBy(common.ContextCol)
	// SortStatus is a key binding for sorting by the "Status" column.
	SortStatus = sortBy(common.StatusCol)
	// SortSize is a key binding for sorting by the "Size" column.
	SortSize = sortBy(common.SizeCol)
	// SortFree is a key binding for sorting by the "Free" column.
	SortFree = sortBy(common.FreeCol)
	// SortAge is a key binding for sorting by the "Age" column.
	SortAge = sortBy(common.AgeCol)
	// SortGPUs is a key binding for sorting by the "GPUs" column. Shared
	// by every category with a GPU-count column: ServingRuntime,
	// GPUPool and GPUWorkload all title theirs "GPUs".
	SortGPUs = sortBy(common.GpusCol)
	// SortUsage is a key binding for sorting by the "Usage" column.
	SortUsage = sortBy(common.UsageCol)
	// ToggleAlias is a key binding for toggling the alias view.
	ToggleAlias = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("^a", "Toggle aliases"),
	)
	// TogglePreview shows or hides the selected row's summary beside the
	// table, on terminals wide enough for both.
	TogglePreview = key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "Toggle preview"),
	)
	// SortType is a key binding for sorting by the "Type" column.
	SortType = sortBy(common.TypeCol)
	// SortVendor is a key binding for sorting by the "Vendor" column.
	SortVendor = sortBy(common.VendorCol)
	// ToggleFaulty is a key binding for toggling the faulty state.
	ToggleFaulty = key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("^z", "Faulty only"),
	)
	// EditTenant opens the tenant-metadata entry form for an
	// unresolved DAC/ImportedModel row.
	EditTenant = key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("E", "Edit tenant"),
	)
)

// Category+mode-specific key bindings
var catContext = map[domain.Category]map[common.ViewMode][]key.Binding{
	domain.BaseModel: {
		common.ListView: {SortSize, SortContext, ToggleFaulty, Refresh},
	},
	domain.Tenant: {
		common.ListView: {SortInternal, SortStatus, CopyTenant, ToggleFaulty},
	},
	domain.ConsolePropertyDefinition: {
		common.ListView: {SortValue},
	},
	domain.PropertyDefinition: {
		common.ListView: {SortValue},
	},
	domain.GPUPool: {
		common.ListView: {SortSize, SortGPUs, ToggleFaulty, ScaleUp, Refresh},
	},
	domain.GPUNode: {
		common.ListView: {GoTo, SortFree, SortType, SortAge, Refresh, ToggleCordon, DrainNode, ToggleFaulty, Restart, Delete},
	},
	domain.GPUWorkload: {
		common.ListView: {GoTo, SortTenant, SortGPUs, SortAge, OpenMetrics, ToggleFaulty, Refresh},
	},
	domain.ServingRuntime: {
		common.ListView: {SortGPUs, ToggleFaulty, Refresh},
	},
	domain.DedicatedAICluster: {
		common.ListView: {GoTo, SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, EditTenant, OpenMetrics, Refresh, ToggleFaulty, Delete},
	},
	domain.InferenceService: {
		common.ListView: {GoTo, SortTenant, SortAge, CopyTenant, OpenMetrics, ToggleFaulty, Refresh, Restart, Delete},
	},
	domain.ImportedModel: {
		common.ListView: {GoTo, SortTenant, SortSize, SortContext, SortVendor, CopyTenant, EditTenant, OpenMetrics, Refresh},
	},
	domain.LimitTenancyOverride: {
		common.ListView: {GoTo, SortTenant, SortRegions, CopyTenant},
	},
	domain.ConsolePropertyTenancyOverride: {
		common.ListView: {GoTo, SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.PropertyTenancyOverride: {
		common.ListView: {GoTo, SortTenant, SortRegions, SortValue, CopyTenant},
	},
	domain.LimitRegionalOverride: {
		common.ListView: {GoTo, SortRegions},
	},
	domain.PropertyRegionalOverride: {
		common.ListView: {GoTo, SortRegions, SortValue},
	},
	domain.ConsolePropertyRegionalOverride: {
		common.ListView: {GoTo, SortRegions, SortValue},
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
	sortKeys := SortKeys(cat)
	km.Mode = withSortKeys(km.Mode, sortKeys)
	km.Context = withSortKeys(km.Context, sortKeys)
	return km
}
