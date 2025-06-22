// Package common provides shared types and utilities for the TUI components.
package common

/*
EditTarget represents the current edit target in the UI.
Implements Stringer for testability and debugging.
*/
type EditTarget int

const (
	// NoneTarget is the zero value for EditTarget, indicating no edit target is selected.
	NoneTarget EditTarget = iota // No edit target selected (zero value)
	// FilterTarget is the edit target for filters.
	FilterTarget // Filter edit target
	// AliasTarget is the edit target for aliases.
	AliasTarget // Alias edit target
)

// String returns the string representation of the EditTarget.
func (e EditTarget) String() string {
	switch e {
	case NoneTarget:
		return "none"
	case FilterTarget:
		return "filter"
	case AliasTarget:
		return "alias"
	default:
		return "unknown"
	}
}
