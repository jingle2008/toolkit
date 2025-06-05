package toolkit

// EditTarget represents the current edit target in the UI.
type EditTarget int

const (
	// None indicates no edit target is selected.
	None EditTarget = iota
	// Filter indicates the filter edit target.
	Filter
	// Alias indicates the alias edit target.
	Alias
)

// StatusMode represents the current mode of the UI.
type StatusMode int

const (
	// Edit indicates the UI is in edit mode.
	Edit StatusMode = iota
	// Normal indicates the UI is in normal mode.
	Normal
)
