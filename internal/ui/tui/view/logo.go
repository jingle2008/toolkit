package view

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Logo is the header's right-end mark: a GPU chip whose die carries a T.
// It is six rows tall to match the info column beside it.
const Logo = `    | | | | |
  .-----------.
 -|   _____   |-
 -|  |_   _|  |-
 -|    |_|    |-
  '-----------'`

// LogoMargin is the blank column kept between the logo and the right edge.
const LogoMargin = 1

// LogoWidth is the rendered width of Logo, excluding LogoMargin.
var LogoWidth = lipgloss.Width(Logo)

// RenderLogo renders Logo with a top-to-bottom gradient between two
// "#RRGGBB" colors. Every row is padded to LogoWidth plus LogoMargin so
// the block keeps its shape when placed flush right.
func RenderLogo(top, bottom lipgloss.Color) string {
	rows := strings.Split(Logo, "\n")
	out := make([]string, len(rows))
	for i, row := range rows {
		t := float64(i) / float64(max(len(rows)-1, 1))
		pad := strings.Repeat(" ", LogoWidth-lipgloss.Width(row)+LogoMargin)
		out[i] = lipgloss.NewStyle().Foreground(blend(top, bottom, t)).Render(row) + pad
	}
	return strings.Join(out, "\n")
}

// blend linearly interpolates two "#RRGGBB" colors; t=0 is a, t=1 is b.
// A malformed input yields a.
func blend(a, b lipgloss.Color, t float64) lipgloss.Color {
	var ar, ag, ab, br, bg, bb int
	if _, err := fmt.Sscanf(string(a), "#%02x%02x%02x", &ar, &ag, &ab); err != nil {
		return a
	}
	if _, err := fmt.Sscanf(string(b), "#%02x%02x%02x", &br, &bg, &bb); err != nil {
		return a
	}
	mix := func(x, y int) int { return x + int(math.Round(float64(y-x)*t)) }
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", mix(ar, br), mix(ag, bg), mix(ab, bb)))
}
