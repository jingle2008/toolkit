package tui

import (
	"image/color"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// Palette names the TUI's colors by role. Views take colors from roles,
// never literals, so a light terminal or a retune changes them in one
// place.
type Palette struct {
	Primary   color.Color // keys, info labels, section headers
	Accent    color.Color // context chip, selected row, spinner, logo bottom
	Secondary color.Color // counters, stats chip
	Text      color.Color // body text
	OnAccent  color.Color // text on Accent/Secondary/Success/Info/Danger fills
	Muted     color.Color // hints, debug lines, dim text
	Border    color.Color // unfocused borders, separators
	Focus     color.Color // modal and details borders
	Surface   color.Color // status bar fill
	OnSurface color.Color // status bar text
	Success   color.Color // LIVE, ready
	Warn      color.Color // warnings, paused log
	OnWarn    color.Color // text on Warn fills
	Danger    color.Color // errors, irreversible actions, faulty
	Info      color.Color // info toasts, following log, help keys
}

// NewPalette returns the role colors for a dark (isDark) or light
// terminal background.
func NewPalette(isDark bool) Palette {
	c := func(light, dark string) color.Color {
		return lipgloss.LightDark(isDark)(lipgloss.Color(light), lipgloss.Color(dark))
	}
	return Palette{
		Primary:   c("#D75F00", "#FF8700"),
		Accent:    c("#D7005F", "#FF5F87"),
		Secondary: c("#8700AF", "#A550DF"),
		Text:      c("#262626", "#E4E4E4"),
		OnAccent:  c("#FFFDF5", "#FFFDF5"),
		Muted:     c("#6C6C6C", "#8A8A8A"),
		Border:    c("#BCBCBC", "#585858"),
		Focus:     c("#5F5FAF", "#5F5FD7"),
		Surface:   c("#D9DCCF", "#353533"),
		OnSurface: c("#343433", "#C1C6B2"),
		Success:   c("#1A7F37", "#2EA043"),
		Warn:      c("#9A6700", "#FFAF00"),
		OnWarn:    c("#FFFDF5", "#000000"),
		Danger:    c("#CF222E", "#D70000"),
		Info:      c("#0969DA", "#0087FF"),
	}
}

// Styles groups immutable lipgloss styles used by the TUI, all built from
// one Palette. Keeping these centralized avoids per-Model recomputation
// and makes tests simpler.
type Styles struct {
	Palette Palette

	// Frame chips: the breadcrumb's category path and row stats, and the
	// env bar's LIVE marker.
	Context lipgloss.Style
	Stats   lipgloss.Style
	Live    lipgloss.Style

	// Logo gradient endpoints: Primary fading to Accent.
	LogoTop    view.RGB
	LogoBottom view.RGB

	// Help view styles
	HelpHeader lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style

	// Component styles, pushed into the bubbles by setStyles.
	Table    table.Styles
	Viewport lipgloss.Style
	Spinner  lipgloss.Style

	// Log overlay: title bar by state, hint line, and entry colors.
	LogFollowing lipgloss.Style
	LogPaused    lipgloss.Style
	LogHint      lipgloss.Style
	LogWarn      lipgloss.Style
	LogError     lipgloss.Style
	LogDebug     lipgloss.Style
}

// DefaultStyles returns the shared immutable style set for the TUI on a
// dark (isDark) or light terminal background.
func DefaultStyles(isDark bool) Styles {
	p := NewPalette(isDark)
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	fill := func(bg, text color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Background(bg).Foreground(text)
	}

	chip := fg(p.OnAccent).Padding(0, 1)

	tbl := table.DefaultStyles()
	tbl.Header = tbl.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Border).
		BorderBottom(true).
		Bold(true)
	tbl.Selected = tbl.Selected.
		Foreground(p.OnAccent).
		Background(p.Accent).
		Bold(true)

	logBar := lipgloss.NewStyle().Bold(true)

	return Styles{
		Palette: p,

		Context:    chip.Background(p.Accent).Bold(true),
		Stats:      chip.Background(p.Secondary).Align(lipgloss.Right),
		Live:       chip.Background(p.Success).Bold(true),
		LogoTop:    toRGB(p.Primary),
		LogoBottom: toRGB(p.Accent),

		HelpHeader: fg(p.Primary).Underline(true),
		HelpKey:    fg(p.Info),
		HelpDesc:   lipgloss.NewStyle(),

		Table:    tbl,
		Viewport: lipgloss.NewStyle(), // the frame's panel draws the border
		Spinner:  fg(p.Accent),

		LogFollowing: logBar.Inherit(fill(p.Info, p.OnAccent)),
		LogPaused:    logBar.Inherit(fill(p.Warn, p.OnWarn)),
		LogHint:      fg(p.Muted),
		LogWarn:      fg(p.Warn),
		LogError:     fg(p.Danger),
		LogDebug:     fg(p.Muted),
	}
}

// toRGB converts c to the logo's 24-bit color.
func toRGB(c color.Color) view.RGB {
	r, g, b, _ := c.RGBA()
	return view.RGB{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)} //nolint:gosec // RGBA channels are 16-bit, so >>8 fits a byte
}
