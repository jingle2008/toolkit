package tui

import "github.com/charmbracelet/lipgloss"

// Styles groups immutable lipgloss styles used by the TUI.
// Keeping these centralized avoids per-Model recomputation and makes tests simpler.
type Styles struct {
	Base         lipgloss.Style
	StatusNugget lipgloss.Style
	StatusBar    lipgloss.Style
	Context      lipgloss.Style
	Stats        lipgloss.Style
	StatusText   lipgloss.Style
	InfoKey      lipgloss.Style
	InfoValue    lipgloss.Style

	// Help view styles
	HelpBorder lipgloss.Style
	HelpHeader lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
}

// DefaultStyles returns the shared immutable style set for the TUI.
func DefaultStyles() Styles {
	base := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	statusNugget := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})

	context := lipgloss.NewStyle().
		Inherit(statusBar).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1)

	stats := statusNugget.
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

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
		StatusText:   statusText,
		InfoKey:      infoKey,
		InfoValue:    infoValue,

		HelpBorder: helpBorder,
		HelpHeader: helpHeader,
		HelpKey:    helpKey,
		HelpDesc:   helpDesc,
	}
}
