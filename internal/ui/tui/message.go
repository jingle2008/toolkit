/*
Package tui defines message types for the TUI model.
*/
package tui

// ErrMsg is a message containing an error.
type ErrMsg struct{ Err error }

// DataMsg is a message containing generic data.
type DataMsg struct{ Data interface{} }

// FilterMsg is a message containing filter text.
type FilterMsg struct{ Text string }

// SetFilterMsg is a message to set the filter text in the model.
type SetFilterMsg struct{ Text string }
