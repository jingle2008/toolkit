package rows

import (
	"github.com/charmbracelet/bubbles/table"
)

/*
RowMarshaler is a generic interface for types that can be marshaled into a table.Row for display.
*/
type RowMarshaler[T any] interface {
	ToRow(scope string) table.Row
}

/*
MarshalRows is a generic helper to convert a slice of RowMarshaler to []table.Row.
*/
func MarshalRows[T RowMarshaler[T]](scope string, items []T) []table.Row {
	rows := make([]table.Row, len(items))
	for i, item := range items {
		rows[i] = item.ToRow(scope)
	}
	return rows
}
