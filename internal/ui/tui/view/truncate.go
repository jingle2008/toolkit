package view

import "github.com/mattn/go-runewidth"

// Truncate fits s within limit cells, ending in "…" (one cell to spare)
// when it doesn't fit.
// s is plain text.
func Truncate(s string, limit int) string {
	if runewidth.StringWidth(s) <= limit {
		return s
	}
	return runewidth.Truncate(s, limit-1, "…")
}

// TruncateMiddle returns s shortened to fit width w by eliding the
// middle: head + "…" + tail, where head and tail are sized to use the
// available width minus the ellipsis. The split favors the tail by one
// cell when w-1 is odd, since the tail is usually what tells values
// apart (an OCID's suffix, a pool's _cn_02, an image's tag). If s
// already fits, it is returned unchanged. s is plain text.
func TruncateMiddle(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	const ellipsis = "…"
	ew := runewidth.StringWidth(ellipsis)
	if w < ew {
		return ""
	}
	if w == ew {
		return ellipsis
	}
	keep := w - ew
	headW := keep / 2
	tailW := keep - headW

	runes := []rune(s)
	headEnd := 0
	var acc int
	for i := 0; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if acc+rw > headW {
			break
		}
		acc += rw
		headEnd = i + 1
	}
	tailStart := len(runes)
	acc = 0
	for i := len(runes) - 1; i >= 0; i-- {
		rw := runewidth.RuneWidth(runes[i])
		if acc+rw > tailW {
			break
		}
		acc += rw
		tailStart = i
	}
	if tailStart < headEnd {
		tailStart = headEnd
	}
	return string(runes[:headEnd]) + ellipsis + string(runes[tailStart:])
}
