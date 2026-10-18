package view

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Dim redraws s in a single muted color, dropping its own styling, so a
// modal composited over it stands out while the view behind stays legible.
func Dim(s string, muted color.Color) string {
	return lipgloss.NewStyle().Foreground(muted).Render(ansi.Strip(s))
}

// Overlay composites fg centered over bg in a w×h screen. Both are
// rendered strings; cells fg covers replace bg's, the rest of bg shows
// through.
func Overlay(bg, fg string, w, h int) string {
	x := max((w-lipgloss.Width(fg))/2, 0)
	y := max((h-lipgloss.Height(fg))/2, 0)
	return compose(bg, fg, x, y)
}

// Corner composites fg near bg's bottom-right corner in a w×h screen,
// inset one row and two columns so it floats inside a full-screen frame
// rather than over its border.
func Corner(bg, fg string, w, h int) string {
	x := max(w-lipgloss.Width(fg)-2, 0)
	y := max(h-lipgloss.Height(fg)-1, 0)
	return compose(bg, fg, x, y)
}

func compose(bg, fg string, x, y int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(bg),
		lipgloss.NewLayer(fg).X(x).Y(y).Z(1),
	).Render()
}

// Box frames body in a rounded border of color border, with title set
// into the top edge: ╭─ Title ───╮. width is the outer width; 0 fits the
// body, widened if need be so the title isn't cut. Body lines get one
// column of padding on each side.
func Box(title, body string, border color.Color, titleStyle lipgloss.Style, width int) string {
	inner := lipgloss.NewStyle().Padding(0, 1)
	switch {
	case width > 0:
		inner = inner.Width(max(width-2, 0))
	case title != "":
		// The top edge needs 6 cells around the title: ╭─ ␣title␣ ─╮.
		inner = inner.Width(max(lipgloss.Width(body)+2, lipgloss.Width(title)+4))
	}
	edge := lipgloss.NewStyle().Foreground(border)
	boxed := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderTop(false).
		Render(inner.Render(body))
	return topEdge(title, lipgloss.Width(boxed), edge, titleStyle) + "\n" + boxed
}

// topEdge draws a w-wide rounded top border with title inset after "╭─ ".
// A title too long for the edge is truncated with an ellipsis.
func topEdge(title string, w int, edge, titleStyle lipgloss.Style) string {
	b := lipgloss.RoundedBorder()
	if title == "" {
		return edge.Render(b.TopLeft + strings.Repeat(b.Top, max(w-2, 0)) + b.TopRight)
	}
	// ╭─ + space + title + space + fill + ╮; fill is at least one ─.
	room := w - 6
	if room < 1 {
		return edge.Render(b.TopLeft + strings.Repeat(b.Top, max(w-2, 0)) + b.TopRight)
	}
	title = ansi.Truncate(title, room, "…")
	fill := w - 5 - lipgloss.Width(title)
	return edge.Render(b.TopLeft+b.Top+" ") +
		titleStyle.Render(title) +
		edge.Render(" "+strings.Repeat(b.Top, fill)+b.TopRight)
}
