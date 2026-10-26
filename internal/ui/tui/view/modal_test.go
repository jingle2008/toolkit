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

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", Truncate("short", 5))
	assert.Equal(t, "sho…", Truncate("shorter", 5), "a cell to spare")
	assert.Equal(t, "日…", Truncate("日本語", 4), "wide runes count two cells")
}
