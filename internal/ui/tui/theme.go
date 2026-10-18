package tui

import (
	"image/color"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// Palette names the TUI's colors by role. Views take colors from roles,
// never literals, so a light terminal or a retune changes them in one
// place. Each hue has one meaning:
//
//   - Primary (orange) marks a key you can press, and nothing else.
//   - Accent marks focus: modal borders, cursors, the selection marker,
//     matched text, the spinner.
//   - Success, Warn and Danger are status; Danger's fill (DangerFill) is
//     reserved for production and irreversible actions.
//   - Selection is the selected row's background: a quiet tint, so the
//     status colors stay readable on it.
//
// Every text role reaches 4.5:1 on common terminal backgrounds and on
// Selection, and every fill 4.5:1 under its text (TestPalette_Contrast).
type Palette struct {
	Primary    color.Color // keys
	Accent     color.Color // focus: modal borders, cursors, selection marker, matches
	Text       color.Color // body text, titles
	Muted      color.Color // hints, labels, dim text
	Border     color.Color // panel borders, separators
	Selection  color.Color // the selected row's background
	Success    color.Color // healthy status, LIVE
	Warn       color.Color // transitional status, warnings, paused log
	OnWarn     color.Color // text on a Warn fill (the focused recoverable button)
	Danger     color.Color // failed status, errors
	DangerFill color.Color // PROD, PRODUCTION, the focused irreversible button
	OnAccent   color.Color // text on DangerFill
}

// NewPalette returns the role colors for a dark (isDark) or light
// terminal background.
func NewPalette(isDark bool) Palette {
	c := func(light, dark string) color.Color {
		return lipgloss.LightDark(isDark)(lipgloss.Color(light), lipgloss.Color(dark))
	}
	return Palette{
		Primary:    c("#A04400", "#FF8F1A"),
		Accent:     c("#5252A3", "#A3A3FF"),
		Text:       c("#262626", "#E4E4E4"),
		Muted:      c("#5C5C5C", "#ADADAD"),
		Border:     c("#BCBCBC", "#585858"),
		Selection:  c("#DCE8F7", "#203A5C"),
		Success:    c("#166B2E", "#4CC85C"),
		Warn:       c("#7A5000", "#FFAF00"),
		OnWarn:     c("#FFFDF5", "#000000"),
		Danger:     c("#B71C26", "#FF8080"),
		DangerFill: c("#CF222E", "#D70000"),
		OnAccent:   c("#FFFDF5", "#FFFDF5"),
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

	// Frame text: the breadcrumb's category path and row stats, the env
	// bar's LIVE marker, and the production badge — the frame's one fill.
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

	// Log: the state chip (following, paused), hint line, entry colors.
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
	pad := lipgloss.NewStyle().Padding(0, 1)

	tbl := table.DefaultStyles()
	tbl.Header = tbl.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Border).
		BorderBottom(true).
		Bold(true)
	// No foreground: cells keep their own colors (statuses, dashes) and
	// the rest the terminal's text color, all readable on Selection.
	tbl.Selected = lipgloss.NewStyle().Background(p.Selection).Bold(true)

	return Styles{
		Palette: p,

		Context:    pad.Foreground(p.Text).Bold(true),
		Stats:      pad.Foreground(p.Muted).Align(lipgloss.Right),
		Live:       pad.Foreground(p.Success).Bold(true),
		Prod:       pad.Background(p.DangerFill).Foreground(p.OnAccent).Bold(true),
		LogoTop:    toRGB(p.Primary),
		LogoBottom: toRGB(p.Accent),

		HelpHeader: lipgloss.NewStyle().Foreground(p.Text).Bold(true),
		HelpKey:    fg(p.Primary),
		HelpDesc:   lipgloss.NewStyle(),

		Table:    tbl,
		Viewport: lipgloss.NewStyle(), // the frame's panel draws the border
		Spinner:  fg(p.Accent),

		LogFollowing: fg(p.Accent).Bold(true),
		LogPaused:    fg(p.Warn).Bold(true),
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

// pickerStyles themes the export folder list, which otherwise draws in
// the filepicker's own pink and purple: folders in Text, the highlighted
// one and its cursor in Accent, files and details Muted.
func pickerStyles(st filepicker.Styles, p Palette) filepicker.Styles {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	st.Cursor = fg(p.Accent)
	st.Selected = fg(p.Accent).Bold(true)
	st.Directory = fg(p.Text)
	st.File = fg(p.Muted)
	st.Symlink = fg(p.Muted)
	st.DisabledCursor = fg(p.Muted)
	st.DisabledFile = fg(p.Muted)
	st.DisabledSelected = fg(p.Muted)
	st.Permission = fg(p.Muted)
	st.FileSize = fg(p.Muted)
	st.EmptyDirectory = st.EmptyDirectory.Foreground(p.Muted)
	return st
}

// toRGB converts c to the logo's 24-bit color.
func toRGB(c color.Color) view.RGB {
	r, g, b, _ := c.RGBA()
	return view.RGB{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)} //nolint:gosec // RGBA channels are 16-bit, so >>8 fits a byte
}
