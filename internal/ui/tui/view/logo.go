package view

import (
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
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

// RGB is a 24-bit color. The logo gradient interpolates on its channels
// rather than parsing color strings, which keeps it independent of how
// the styling library represents colors.
type RGB struct{ R, G, B uint8 }

// Hex formats c as "#RRGGBB".
func (c RGB) Hex() string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// RenderLogo renders Logo with a top-to-bottom gradient from top to
// bottom. Every row is padded to LogoWidth plus LogoMargin so the block
// keeps its shape when placed flush right.
func RenderLogo(top, bottom RGB) string {
	rows := strings.Split(Logo, "\n")
	out := make([]string, len(rows))
	for i, row := range rows {
		t := float64(i) / float64(max(len(rows)-1, 1))
		pad := strings.Repeat(" ", LogoWidth-lipgloss.Width(row)+LogoMargin)
		fg := lipgloss.Color(blend(top, bottom, t).Hex())
		out[i] = lipgloss.NewStyle().Foreground(fg).Render(row) + pad
	}
	return strings.Join(out, "\n")
}

// blend linearly interpolates two colors; t=0 is a, t=1 is b.
func blend(a, b RGB, t float64) RGB {
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return RGB{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B)}
}
