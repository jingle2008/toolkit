package tui

import (
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// Styles groups immutable lipgloss styles used by the TUI.
// Keeping these centralized avoids per-Model recomputation and makes tests simpler.
type Styles struct {
	Base         lipgloss.Style
	StatusNugget lipgloss.Style
	StatusBar    lipgloss.Style
	Context      lipgloss.Style
	Stats        lipgloss.Style
	Live         lipgloss.Style
	StatusText   lipgloss.Style
	InfoKey      lipgloss.Style
	InfoValue    lipgloss.Style

	// Header logo gradient endpoints: the info-key orange
	// fading to the status bar's context pink.
	LogoTop    view.RGB
	LogoBottom view.RGB

	// Help view styles
	HelpBorder lipgloss.Style
	HelpHeader lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
}

// DefaultStyles returns the shared immutable style set for the TUI on a
// dark (isDark) or light terminal background.
func DefaultStyles(isDark bool) Styles {
	lightDark := lipgloss.LightDark(isDark)

	base := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	statusNugget := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	statusBar := lipgloss.NewStyle().
		Foreground(lightDark(lipgloss.Color("#343433"), lipgloss.Color("#C1C6B2"))).
		Background(lightDark(lipgloss.Color("#D9DCCF"), lipgloss.Color("#353533")))

	context := lipgloss.NewStyle().
		Inherit(statusBar).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1)

	stats := statusNugget.
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

	live := statusNugget.
		Background(lipgloss.Color("#2EA043")).
		Bold(true)

	statusText := lipgloss.NewStyle().Inherit(statusBar)
	infoKey := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	infoValue := lipgloss.NewStyle().Width(30)

	helpBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)
	helpHeader := lipgloss.NewStyle().Inherit(infoKey).Underline(true)
	helpKey := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	helpDesc := lipgloss.NewStyle()

	return Styles{
		Base:         base,
		StatusNugget: statusNugget,
		StatusBar:    statusBar,
		Context:      context,
		Stats:        stats,
		Live:         live,
		StatusText:   statusText,
		InfoKey:      infoKey,
		InfoValue:    infoValue,
		LogoTop:      view.RGB{R: 0xFF, G: 0x87, B: 0x00}, // xterm 208, as infoKey
		LogoBottom:   view.RGB{R: 0xFF, G: 0x5F, B: 0x87}, // as context

		HelpBorder: helpBorder,
		HelpHeader: helpHeader,
		HelpKey:    helpKey,
		HelpDesc:   helpDesc,
	}
}

// boxWidth sets s's width to w plus its horizontal border. lipgloss v2
// counts the border inside Width, v1 added it outside; this keeps the
// boxes' v1 outer width.
func boxWidth(s lipgloss.Style, w int) lipgloss.Style {
	return s.Width(w + s.GetHorizontalBorderSize())
}
