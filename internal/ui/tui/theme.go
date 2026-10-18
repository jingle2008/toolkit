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
//
// A role drawn as text and one drawn as a fill behind OnAccent text need
// different shades on a dark background — a color bright enough to read
// as text is too bright to carry white text — so the *Fill roles hold
// the fills. Every text role reaches 4.5:1 on common terminal
// backgrounds and every fill 4.5:1 under OnAccent (TestPalette_Contrast).
type Palette struct {
	Primary   color.Color // keys, info labels, section headers
	Accent    color.Color // spinner, logo bottom
	Secondary color.Color // breadcrumb chips
	Text      color.Color // body text
	OnAccent  color.Color // text on the *Fill roles
	Muted     color.Color // hints, debug lines, dim text
	Border    color.Color // unfocused borders, separators
	Focus     color.Color // modal and details borders
	Surface   color.Color // status bar fill
	OnSurface color.Color // status bar text
	Success   color.Color // LIVE, ready
	Warn      color.Color // warnings, paused log
	OnWarn    color.Color // text on Warn fills
	Danger    color.Color // errors, irreversible actions, faulty
	Info      color.Color // info toasts, help keys

	AccentFill    color.Color // selected row, context chip
	SecondaryFill color.Color // stats chip, picker badges
	SuccessFill   color.Color // LIVE
	DangerFill    color.Color // PROD badge, irreversible confirm button
	InfoFill      color.Color // following log
}

// NewPalette returns the role colors for a dark (isDark) or light
// terminal background.
func NewPalette(isDark bool) Palette {
	c := func(light, dark string) color.Color {
		return lipgloss.LightDark(isDark)(lipgloss.Color(light), lipgloss.Color(dark))
	}
	return Palette{
		Primary:   c("#B54D00", "#FF8700"),
		Accent:    c("#D7005F", "#FF5F87"),
		Secondary: c("#8700AF", "#BC8CFF"),
		Text:      c("#262626", "#E4E4E4"),
		OnAccent:  c("#FFFDF5", "#FFFDF5"),
		Muted:     c("#6C6C6C", "#9E9E9E"),
		Border:    c("#BCBCBC", "#585858"),
		Focus:     c("#5F5FAF", "#8787FF"),
		Surface:   c("#D9DCCF", "#353533"),
		OnSurface: c("#343433", "#C1C6B2"),
		Success:   c("#1A7F37", "#3FB950"),
		Warn:      c("#8A5A00", "#FFAF00"),
		OnWarn:    c("#FFFDF5", "#000000"),
		Danger:    c("#CF222E", "#FF6B6B"),
		Info:      c("#0969DA", "#58A6FF"),

		AccentFill:    c("#D7005F", "#C2185B"),
		SecondaryFill: c("#8700AF", "#8250DF"),
		SuccessFill:   c("#1A7F37", "#238636"),
		DangerFill:    c("#CF222E", "#D70000"),
		InfoFill:      c("#0969DA", "#1F6FEB"),
	}
}

// Styles groups immutable lipgloss styles used by the TUI, all built from
// one Palette. Keeping these centralized avoids per-Model recomputation
// and makes tests simpler.
type Styles struct {
	Palette Palette
	// Monochrome marks a terminal without color (NO_COLOR, or no
	// support): focus is drawn in reverse video, since its fills vanish.
	Monochrome bool

	// Frame chips: the breadcrumb's category path and row stats, and the
	// env bar's LIVE marker and production badge.
	Context lipgloss.Style
	Stats   lipgloss.Style
	Live    lipgloss.Style
	Prod    lipgloss.Style

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
		Background(p.AccentFill).
		Bold(true)

	logBar := lipgloss.NewStyle().Bold(true)

	return Styles{
		Palette: p,

		Context:    chip.Background(p.AccentFill).Bold(true),
		Stats:      chip.Background(p.SecondaryFill).Align(lipgloss.Right),
		Live:       chip.Background(p.SuccessFill).Bold(true),
		Prod:       chip.Background(p.DangerFill).Bold(true),
		LogoTop:    toRGB(p.Primary),
		LogoBottom: toRGB(p.Accent),

		HelpHeader: fg(p.Primary).Underline(true),
		HelpKey:    fg(p.Info),
		HelpDesc:   lipgloss.NewStyle(),

		Table:    tbl,
		Viewport: lipgloss.NewStyle(), // the frame's panel draws the border
		Spinner:  fg(p.Accent),

		LogFollowing: logBar.Inherit(fill(p.InfoFill, p.OnAccent)),
		LogPaused:    logBar.Inherit(fill(p.Warn, p.OnWarn)),
		LogHint:      fg(p.Muted),
		LogWarn:      fg(p.Warn),
		LogError:     fg(p.Danger),
		LogDebug:     fg(p.Muted),
	}
}

// monochrome adapts s to a terminal without color, where fills and
// foregrounds are dropped: the selected row is drawn in reverse video so
// it still stands out.
func (s Styles) monochrome() Styles {
	s.Monochrome = true
	s.Table.Selected = lipgloss.NewStyle().Bold(true).Reverse(true)
	return s
}

// toRGB converts c to the logo's 24-bit color.
func toRGB(c color.Color) view.RGB {
	r, g, b, _ := c.RGBA()
	return view.RGB{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)} //nolint:gosec // RGBA channels are 16-bit, so >>8 fits a byte
}
