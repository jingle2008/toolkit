package tui

import "math"

// longColumnFloor is the narrowest a column with long values (names,
// OCIDs) shrinks to before columns start dropping; shorter values keep
// their full width.
const longColumnFloor = 12

// minTitleWidth is how much of a header survives when its column's
// values are narrower than the header ("Inte…" for booleans).
const minTitleWidth = 4

// columnSpec is what columnWidths needs to know about one column.
type columnSpec struct {
	title   int     // header width, sort arrow included
	content int     // widest value
	ratio   float64 // share of spare width, from the column registry
	keep    bool    // never dropped: the identifying first column, Status
	whole   bool    // never shrunk below its need: Status, the row's health
}

/*
columnWidths sizes columns to fit avail cells, each column also costing
pad cells of padding. A column needs its header or its widest value,
whichever is wider. When every need fits, the spare width is shared by
ratio. When it doesn't, columns shrink toward a floor — short values keep
their width, long ones give way down to longColumnFloor, and whole ones
(Status) don't shrink at all — sharing what
there is by ratio. When even the floors don't fit, columns drop from the
right, sparing the kept ones; a dropped column has width 0, which the
table skips. When the kept ones alone don't fit, the first column shrinks
past its floor so Status stays whole.
*/
func columnWidths(cols []columnSpec, avail, pad int) []int {
	need := make([]int, len(cols))
	floor := make([]int, len(cols))
	visible := make([]bool, len(cols))
	for i, c := range cols {
		need[i] = max(c.title, c.content, 1)
		floor[i] = min(need[i], max(min(c.content, longColumnFloor), min(c.title, minTitleWidth), 1))
		if c.whole {
			floor[i] = need[i]
		}
		visible[i] = true
	}
	room := func() int { return avail - pad*count(visible) }
	for room() < sumVisible(floor, visible) {
		if !dropRightmost(cols, visible) {
			break // only kept columns left
		}
	}
	// The kept columns alone still don't fit (a narrow terminal and a long
	// Status, "ERROR: Unhealthy"): the ones that may shrink give up their
	// floor, down to one cell each, so Status stays whole and the row stays
	// inside the panel instead of pushing its border off screen.
	// A table not sized yet (avail 0) has nothing to fit into: leave it.
	if deficit := sumVisible(floor, visible) - room(); deficit > 0 && avail > 0 {
		for i := range cols {
			if visible[i] && !cols[i].whole {
				cut := min(deficit, floor[i]-1)
				floor[i] -= cut
				deficit -= cut
			}
		}
	}

	widths := make([]int, len(cols))
	if sumVisible(need, visible) <= room() {
		copyVisible(widths, need, visible)
		share(widths, cols, visible, room()-sumVisible(need, visible), nil)
		return widths
	}
	copyVisible(widths, floor, visible)
	share(widths, cols, visible, max(room()-sumVisible(floor, visible), 0), need)
	return widths
}

// count is how many columns are visible.
func count(visible []bool) int {
	n := 0
	for _, v := range visible {
		if v {
			n++
		}
	}
	return n
}

// sumVisible totals vals over the visible columns.
func sumVisible(vals []int, visible []bool) int {
	t := 0
	for i, v := range vals {
		if visible[i] {
			t += v
		}
	}
	return t
}

// copyVisible sets dst to src for the visible columns.
func copyVisible(dst, src []int, visible []bool) {
	for i := range src {
		if visible[i] {
			dst[i] = src[i]
		}
	}
}

// dropRightmost hides the rightmost visible column that isn't kept and
// reports whether there was one.
func dropRightmost(cols []columnSpec, visible []bool) bool {
	for i := len(cols) - 1; i >= 0; i-- {
		if visible[i] && !cols[i].keep {
			visible[i] = false
			return true
		}
	}
	return false
}

// share hands spare cells to the visible columns in proportion to their
// ratios, never past limit when one is given; cells that rounding leaves
// over go one at a time from the left.
func share(widths []int, cols []columnSpec, visible []bool, spare int, limit []int) {
	open := func(i int) bool { return visible[i] && (limit == nil || widths[i] < limit[i]) }
	room := func(i int) int {
		if limit == nil {
			return spare
		}
		return limit[i] - widths[i]
	}
	for spare > 0 {
		given := shareByRatio(widths, cols, spare, open, room)
		if given == 0 {
			given = shareOneEach(widths, len(cols), spare, open)
		}
		if given == 0 {
			return
		}
		spare -= given
	}
}

// shareByRatio gives each open column its ratio's share of spare, up to
// its room, and returns how many cells it gave.
func shareByRatio(widths []int, cols []columnSpec, spare int, open func(int) bool, room func(int) int) int {
	var total float64
	for i := range cols {
		if open(i) {
			total += max(cols[i].ratio, 0.01)
		}
	}
	given := 0
	for i := range cols {
		if !open(i) {
			continue
		}
		add := min(int(math.Floor(float64(spare)*max(cols[i].ratio, 0.01)/total)), room(i))
		widths[i] += add
		given += add
	}
	return given
}

// shareOneEach gives one cell to each open column from the left until
// spare runs out, and returns how many it gave.
func shareOneEach(widths []int, n, spare int, open func(int) bool) int {
	given := 0
	for i := 0; i < n && given < spare; i++ {
		if open(i) {
			widths[i]++
			given++
		}
	}
	return given
}
