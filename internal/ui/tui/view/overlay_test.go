package view

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grid(w, h int, r string) string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = strings.Repeat(r, w)
	}
	return strings.Join(rows, "\n")
}

func TestOverlay_CentersAndKeepsBackground(t *testing.T) {
	t.Parallel()
	out := strings.Split(ansi.Strip(Overlay(grid(10, 5, "."), "AB\nCD", 10, 5)), "\n")
	require.Len(t, out, 5)
	assert.Equal(t, "..........", out[0])
	assert.Equal(t, "....AB....", out[1])
	assert.Equal(t, "....CD....", out[2])
	assert.Equal(t, "..........", out[4])
}

// Wide runes on either side of the modal edge keep the row's width.
func TestOverlay_WideRunes(t *testing.T) {
	t.Parallel()
	bg := grid(5, 3, "日") // 10 cells wide
	out := strings.Split(ansi.Strip(Overlay(bg, "xy", 10, 3)), "\n")
	require.Len(t, out, 3)
	for i, row := range out {
		assert.Equal(t, 10, ansi.StringWidth(row), "row %d: %q", i, row)
	}
	assert.Contains(t, out[1], "xy")
}

// ANSI styling inside the modal survives compositing.
func TestOverlay_KeepsModalStyling(t *testing.T) {
	t.Parallel()
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("hi")
	out := Overlay(grid(6, 3, "."), red, 6, 3)
	assert.Contains(t, out, "38;2;255;0;0")
}

func TestCorner_BottomRight(t *testing.T) {
	t.Parallel()
	out := strings.Split(ansi.Strip(Corner(grid(8, 4, "."), "ok", 8, 4)), "\n")
	require.Len(t, out, 4)
	assert.Equal(t, "....ok..", out[2])
	assert.Equal(t, "........", out[3])
}

func TestDim_StripsStyling(t *testing.T) {
	t.Parallel()
	in := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#00FF00")).Render("x")
	out := Dim(in, color.Gray{Y: 0x80})
	assert.Equal(t, "x", ansi.Strip(out))
	assert.NotContains(t, out, "48;2;0;255;0")
}

func TestBox_TitleInTopEdge(t *testing.T) {
	t.Parallel()
	out := strings.Split(ansi.Strip(Box("Go to", "row one\nrow two", color.White, lipgloss.NewStyle(), 0)), "\n")
	require.Len(t, out, 4)
	assert.Equal(t, "╭─ Go to ─╮", out[0])
	assert.Equal(t, "│ row one │", out[1])
	assert.Equal(t, "╰─────────╯", out[3])
	for i, row := range out {
		assert.Equal(t, ansi.StringWidth(out[0]), ansi.StringWidth(row), "row %d", i)
	}
}

func TestBox_FixedWidthAndLongTitle(t *testing.T) {
	t.Parallel()
	out := strings.Split(ansi.Strip(Box("a very long title indeed", "x", color.White, lipgloss.NewStyle(), 14)), "\n")
	for i, row := range out {
		assert.Equal(t, 14, ansi.StringWidth(row), "row %d: %q", i, row)
	}
	assert.True(t, strings.HasPrefix(out[0], "╭─ a very … ─╮"), out[0])
}

// A fitted box widens for a title longer than its body.
func TestBox_FitsTitle(t *testing.T) {
	t.Parallel()
	out := strings.Split(ansi.Strip(Box("a longer title", "x", color.White, lipgloss.NewStyle(), 0)), "\n")
	assert.Equal(t, "╭─ a longer title ─╮", out[0])
	assert.Equal(t, ansi.StringWidth(out[0]), ansi.StringWidth(out[1]))
}
