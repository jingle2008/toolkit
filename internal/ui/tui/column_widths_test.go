package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func sum(ws []int) int {
	t := 0
	for _, w := range ws {
		t += w
	}
	return t
}

// When every column's need fits, each gets at least its need and the
// spare goes by ratio; the widths fill the space exactly.
func TestColumnWidths_RoomForEveryNeed(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 20, ratio: 0.5, keep: true}, // Name
		{title: 8, content: 5, ratio: 0.1},              // Internal: header wider than "false"
		{title: 3, content: 3, ratio: 0.1},              // Age
		{title: 6, content: 6, ratio: 0.3, keep: true},  // Status
	}
	got := columnWidths(cols, 100, 2)
	assert.Equal(t, 100-4*2, sum(got))
	for i, c := range cols {
		assert.GreaterOrEqual(t, got[i], max(c.title, c.content), "column %d", i)
	}
	assert.Greater(t, got[0]-20, got[2]-3, "spare follows ratio")
}

// When needs don't fit, short values keep their width and long values
// shrink, never below longColumnFloor while there is room for it.
func TestColumnWidths_LongColumnsGiveWay(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 60, ratio: 0.4, keep: true}, // a long OCID name
		{title: 5, content: 1, ratio: 0.1},              // Total "8"
		{title: 4, content: 1, ratio: 0.1},              // Free
		{title: 6, content: 14, ratio: 0.4, keep: true}, // Status "WARN: CORDONED"
	}
	got := columnWidths(cols, 60, 2)
	assert.Equal(t, 60-4*2, sum(got))
	assert.Equal(t, []int{5, 4}, got[1:3], "short columns keep their header")
	assert.Equal(t, 14, got[3], "Status keeps its value")
	assert.GreaterOrEqual(t, got[0], longColumnFloor)
	assert.Less(t, got[0], 60)
}

// A header wider than every value shrinks to minTitleWidth before
// anything drops.
func TestColumnWidths_HeadersTruncateBeforeColumnsDrop(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 4, ratio: 0.5, keep: true},
		{title: 8, content: 1, ratio: 0.5}, // "Internal" over "✓"-wide values
	}
	got := columnWidths(cols, 4+minTitleWidth+2*2, 2)
	assert.Equal(t, []int{4, minTitleWidth}, got)
}

// When even the floors don't fit, columns drop from the right, but the
// kept ones (first column, Status) never do.
func TestColumnWidths_DropsFromTheRightSparingKept(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 10, ratio: 0.3, keep: true},
		{title: 6, content: 6, ratio: 0.2},
		{title: 4, content: 4, ratio: 0.2},
		{title: 3, content: 3, ratio: 0.1},
		{title: 6, content: 6, ratio: 0.2, keep: true},
	}
	got := columnWidths(cols, 30, 2)
	assert.Equal(t, 0, got[3], "rightmost unkept column drops first")
	assert.Equal(t, 0, got[2])
	assert.Positive(t, got[1])
	assert.Positive(t, got[0])
	assert.Positive(t, got[4], "Status is kept")
	visible := 0
	for _, w := range got {
		if w > 0 {
			visible++
		}
	}
	assert.LessOrEqual(t, sum(got)+visible*2, 30)
}

// With nothing droppable left, the kept columns still get their floors.
func TestColumnWidths_OnlyKeptColumnsLeft(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 30, ratio: 0.5, keep: true},
		{title: 6, content: 6, ratio: 0.5, keep: true},
	}
	got := columnWidths(cols, 10, 2)
	assert.Equal(t, []int{longColumnFloor, 6}, got)
}

// A whole column (Status) keeps its full value; a droppable column goes
// instead.
func TestColumnWidths_WholeColumnDoesNotShrink(t *testing.T) {
	t.Parallel()
	cols := []columnSpec{
		{title: 4, content: 9, ratio: 0.3, keep: true},
		{title: 4, content: 13, ratio: 0.3},
		{title: 3, content: 3, ratio: 0.1},
		{title: 6, content: 14, ratio: 0.3, keep: true, whole: true},
	}
	got := columnWidths(cols, 9+12+14+3*2, 2)
	assert.Equal(t, 14, got[3])
	assert.Equal(t, 0, got[2], "Age-like column drops rather than Status shrinking")
}
