package rows

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

type testRow struct {
	val string
}

func (t testRow) ToRow(scope string) table.Row {
	return table.Row{scope, t.val}
}

func TestMarshalRows(t *testing.T) {
	items := []testRow{
		{val: "foo"},
		{val: "bar"},
	}
	scope := "myscope"
	rows := MarshalRows(scope, items)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][0] != scope || rows[0][1] != "foo" {
		t.Errorf("unexpected row[0]: %v", rows[0])
	}
	if rows[1][0] != scope || rows[1][1] != "bar" {
		t.Errorf("unexpected row[1]: %v", rows[1])
	}
}
