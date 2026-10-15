package view

import "github.com/charmbracelet/lipgloss"

// Logo is the header's right-end mark: a T drawn as a hammer. It is six
// rows tall to match the info column beside it.
const Logo = ` ___________
[___________]
     | |
     | |
     | |
     |_|`

// LogoMargin is the blank column kept between the logo and the right edge.
const LogoMargin = 1

// LogoWidth is the rendered width of Logo, excluding LogoMargin.
var LogoWidth = lipgloss.Width(Logo)
