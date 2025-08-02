/*
Package tui defines message types for the TUI model.
*/
package tui

import "github.com/jingle2008/toolkit/pkg/models"

// ErrMsg is a message containing an error.
type ErrMsg error

// DataMsg is a message containing generic data.
type DataMsg struct{ Data any }

// FilterMsg is a message containing filter text.
type FilterMsg string

// SetFilterMsg is a message to set the filter text in the model.
type SetFilterMsg string

type deleteDoneMsg struct {
	key models.ItemKey
}

type deleteErrMsg struct {
	err       error
	key       models.ItemKey
	prevState string
}
