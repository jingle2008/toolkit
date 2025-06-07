// Package toolkit defines message types for the TUI model.
package tui

// errMsg is a message containing an error.
type ErrMsg struct{ Err error }

// dataMsg is a message containing generic data.
type DataMsg struct{ Data interface{} }

// filterMsg is a message containing filter text.
type FilterMsg struct{ Text string }
