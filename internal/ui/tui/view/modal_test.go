package view

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestModal_HintsOnLastLine(t *testing.T) {
	t.Parallel()
	out := ansi.Strip(Modal("Title", "body", "y confirm", color.White, color.Black, 30))
	lines := strings.Split(out, "\n")
	assert.Contains(t, lines[0], "Title")
	assert.Contains(t, lines[len(lines)-2], "y confirm", "hints sit just above the bottom edge")
	assert.Empty(t, strings.Trim(lines[len(lines)-3], "│ "), "after a blank line")

	bare := ansi.Strip(Modal("Title", "body", "", color.White, color.Black, 30))
	assert.Len(t, strings.Split(bare, "\n"), 3, "no hints, no hint line")
}

// FixedModal pads the body to the unfiltered size, and leaves a larger
// body as it is.
func TestFixedModal_PadsToSize(t *testing.T) {
	t.Parallel()
	small := ansi.Strip(FixedModal("T", "x", "", color.White, color.Black, 0, 20, 5))
	lines := strings.Split(small, "\n")
	assert.Len(t, lines, 5+2, "five body rows inside the border")
	assert.Equal(t, 20+4, ansi.StringWidth(lines[0]), "twenty columns plus border and padding")

	big := FixedModal("T", "a\nb\nc", "", color.White, color.Black, 0, 1, 1)
	assert.Equal(t, Modal("T", "a\nb\nc", "", color.White, color.Black, 0), big)
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", Truncate("short", 5))
	assert.Equal(t, "sho…", Truncate("shorter", 5), "a cell to spare")
	assert.Equal(t, "日…", Truncate("日本語", 4), "wide runes count two cells")
}
