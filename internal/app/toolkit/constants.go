package toolkit

// EditTarget represents the current edit target in the UI.
type EditTarget int

const (
	None EditTarget = iota
	Filter
	Alias
)

// StatusMode represents the current mode of the UI.
type StatusMode int

const (
	Edit StatusMode = iota
	Normal
)
