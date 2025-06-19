package common

// EditTarget represents the current edit target in the UI.
type EditTarget int

const (
	NoneTarget   EditTarget = iota // No edit target selected (zero value)
	FilterTarget                   // Filter edit target
	AliasTarget                    // Alias edit target
)
