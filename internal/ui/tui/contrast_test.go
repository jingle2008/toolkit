package tui

import (
	"image/color"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/mutation"
)

// Every text role reads at 4.5:1 on common terminal backgrounds and on
// the selected row's Selection tint, the tint stands out from the
// background, and every fill carries its text at 4.5:1, in both themes.
func TestPalette_Contrast(t *testing.T) {
	t.Parallel()
	hex := lipgloss.Color
	for _, theme := range []struct {
		name string
		dark bool
		bgs  []color.Color // black/#1E1E1E/One Dark/navy; white/#F5F5F5/Solarized light
	}{
		{"dark", true, []color.Color{hex("#000000"), hex("#1E1E1E"), hex("#282C34"), hex("#011423")}},
		{"light", false, []color.Color{hex("#FFFFFF"), hex("#F5F5F5"), hex("#FDF6E3")}},
	} {
		p := NewPalette(theme.dark)
		text := map[string]color.Color{
			"Primary": p.Primary, "Accent": p.Accent, "Text": p.Text, "Muted": p.Muted,
			"Success": p.Success, "Warn": p.Warn, "Danger": p.Danger,
		}
		for name, c := range text {
			for _, bg := range append(slices.Clone(theme.bgs), p.Selection) {
				assert.GreaterOrEqual(t, contrast(c, bg), minContrast, "%s %s on %v", theme.name, name, bg)
			}
		}
		for _, bg := range theme.bgs {
			assert.GreaterOrEqual(t, contrast(p.Selection, bg), minSelectionContrast,
				"%s selection tint stands out from %v", theme.name, bg)
		}
		fills := map[string][2]color.Color{
			"DangerFill": {p.OnAccent, p.DangerFill},
			"Warn":       {p.OnWarn, p.Warn},
		}
		for name, pair := range fills {
			assert.GreaterOrEqual(t, contrast(pair[0], pair[1]), minContrast, "%s text on %s", theme.name, name)
		}
	}
}

// minSelectionContrast is how far the selection tint must stand from the
// background to read as a band: quiet, but visible.
const minSelectionContrast = 1.1

// The contrast helper agrees with the WCAG reference values.
func TestContrast_Reference(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 21.0, contrast(lipgloss.Color("#000000"), lipgloss.Color("#FFFFFF")), 0.01)
	assert.InDelta(t, 1.0, contrast(lipgloss.Color("#777777"), lipgloss.Color("#777777")), 0.01)
}

// Without color (NO_COLOR, or no support) focus is drawn in reverse
// video — the selected row and the focused confirm button — and stays so
// when the background is reported later.
func TestColorless_FocusInReverse(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.updateLayout(120, 40)
	assert.False(t, m.styles.Selected.GetReverse(), "colors draw the selection")

	_, _ = m.Update(tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	assert.True(t, m.theme.Monochrome)
	assert.True(t, m.styles.Selected.GetReverse())

	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	assert.True(t, m.styles.Selected.GetReverse(), "a later background report keeps it")

	m.requestConfirm(confirmRequest{action: mutation.DrainNode, target: "n1", run: func(int) tea.Cmd { return nil }})
	assert.Contains(t, m.confirmView(), "\x1b[1;7m", "the focused button is bold reverse")

	_, _ = m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	assert.False(t, m.styles.Selected.GetReverse(), "color comes back")
}

// themes are common terminal color schemes, as foreground and background.
var themes = []struct {
	name   string
	dark   bool
	fg, bg string
}{
	{"black", true, "#E4E4E4", "#000000"},
	{"VS Code dark", true, "#D4D4D4", "#1E1E1E"},
	{"One Dark", true, "#ABB2BF", "#282C34"},
	{"neon navy", true, "#CBE0F0", "#011423"},
	{"white", false, "#262626", "#FFFFFF"},
	{"GitHub light", false, "#24292F", "#F5F5F5"},
	{"Solarized light", false, "#657B83", "#FDF6E3"},
}

// A palette derived from a theme keeps every text role readable: the
// hues and Muted on the background and the selection, Text on the
// selection whenever the theme's own text is readable on its background.
// The selection still stands out where any tint is readable.
func TestDerivePalette_Contrast(t *testing.T) {
	t.Parallel()
	for _, th := range themes {
		fg, bg := lipgloss.Color(th.fg), lipgloss.Color(th.bg)
		p := DerivePalette(th.dark, fg, bg)
		text := map[string]color.Color{
			"Primary": p.Primary, "Accent": p.Accent, "Success": p.Success, "Warn": p.Warn, "Danger": p.Danger,
		}
		if contrast(fg, bg) >= minContrast {
			text["Text"] = p.Text
			text["Muted"] = p.Muted
		}
		for name, c := range text {
			assert.GreaterOrEqual(t, contrast(c, bg), minContrast, "%s: %s on background", th.name, name)
			assert.GreaterOrEqual(t, contrast(c, p.Selection), minContrast, "%s: %s on selection", th.name, name)
		}
		if p.Selection != bg {
			assert.GreaterOrEqual(t, contrast(p.Selection, bg), minSelectionContrast, "%s: selection stands out", th.name)
		}
	}
}
