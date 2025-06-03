package toolkit

import (
	"github.com/charmbracelet/bubbles/table"
)

type RowMarshaler interface {
	ToRow(scope string) table.Row
}
