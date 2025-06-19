package common

// InputMode represents the current mode of the UI.
type InputMode int

const (
	// UnknownInput is the zero value for InputMode, indicating an unset or unknown state.
	UnknownInput InputMode = iota // Unset/unknown state (zero value)
	// EditInput is the input mode for editing.
	EditInput // Edit mode
	// NormalInput is the input mode for normal (non-editing) operations.
	NormalInput // Normal mode
)
