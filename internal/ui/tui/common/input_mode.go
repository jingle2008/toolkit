package common

// InputMode represents the current mode of the UI.
type InputMode int

const (
	UnknownInput InputMode = iota // Unset/unknown state (zero value)
	EditInput                     // Edit mode
	NormalInput                   // Normal mode
)
