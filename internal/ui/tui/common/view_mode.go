package common //nolint:revive // shared TUI types

// ViewMode represents the current UI mode (e.g., list, details).
type ViewMode int

const (
	// ListView is the default view mode, displaying a list of items.
	ListView ViewMode = iota
	// DetailsView is the view mode for displaying item details.
	DetailsView
	// LoadingView is the view mode for displaying loading state.
	LoadingView
	// HelpView is the view mode for displaying help information.
	HelpView
	// ExportView is the view mode for exporting table data as CSV.
	ExportView
	// EditTenantView is the view mode for the tenant-metadata entry form.
	EditTenantView
	// LogView is the full-screen log overlay.
	LogView
)

// String returns the string representation of the ViewMode.
func (v ViewMode) String() string {
	switch v {
	case ListView:
		return "List"
	case DetailsView:
		return "Details"
	case LoadingView:
		return "Loading"
	case HelpView:
		return "Help"
	case ExportView:
		return "Export"
	case EditTenantView:
		return "EditTenant"
	case LogView:
		return "Log"
	default:
		return "Unknown"
	}
}
