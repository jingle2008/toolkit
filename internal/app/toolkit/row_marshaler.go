package toolkit

import (
	"github.com/charmbracelet/bubbles/table"
)

/*
RowMarshaler is an interface for types that can be marshaled into a table.Row for display.
*/
type RowMarshaler interface {
	ToRow(scope string) table.Row
}
