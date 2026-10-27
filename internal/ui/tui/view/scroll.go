package view

// ScrollWindow returns the first visible row of a window of n rows that
// keeps cursor in view, moving offset as little as possible.
func ScrollWindow(cursor, offset, n int) int {
	switch {
	case cursor < offset:
		return cursor
	case cursor >= offset+n:
		return cursor - n + 1
	}
	return offset
}
