package common

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
	// ErrorView is the view mode for displaying error state.
	ErrorView
)
