package toolkit

// EditTarget represents the type of edit operation in the toolkit UI.
type EditTarget int

// EditTarget constants enumerate all possible edit targets.
const (
	None EditTarget = iota
	Filter
	Alias
)

// StatusMode represents the current mode of the toolkit UI.
type StatusMode int

// StatusMode constants enumerate all possible status modes.
const (
	Edit StatusMode = iota
	Normal
)
