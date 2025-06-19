package common

// ViewMode represents the current UI mode (e.g., list, details).
type ViewMode int

const (
	ListView ViewMode = iota
	DetailsView
	LoadingView
	HelpView
	ErrorView
)
