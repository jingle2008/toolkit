package tui

import (
	"image/color"
	"math"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/mutation"
)

// luminance is c's WCAG 2 relative luminance.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		s := float64(v>>8) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// contrast is the WCAG 2 contrast ratio of a and b.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// minContrast is WCAG AA for normal text.
const minContrast = 4.5

// Every text role reads at 4.5:1 on common terminal backgrounds, and every
// fill carries its text at 4.5:1, in both themes.
func TestPalette_Contrast(t *testing.T) {
	t.Parallel()
	hex := lipgloss.Color
	for _, theme := range []struct {
		name string
		dark bool
		bgs  []color.Color // black/#1E1E1E/One Dark; white/#F5F5F5/Solarized light
	}{
		{"dark", true, []color.Color{hex("#000000"), hex("#1E1E1E"), hex("#282C34")}},
		{"light", false, []color.Color{hex("#FFFFFF"), hex("#F5F5F5"), hex("#FDF6E3")}},
	} {
		p := NewPalette(theme.dark)
		text := map[string]color.Color{
			"Primary": p.Primary, "Accent": p.Accent, "Secondary": p.Secondary, "Text": p.Text,
			"Muted": p.Muted, "Focus": p.Focus, "Success": p.Success, "Warn": p.Warn,
			"Danger": p.Danger, "Info": p.Info,
		}
		for name, c := range text {
			for _, bg := range theme.bgs {
				assert.GreaterOrEqual(t, contrast(c, bg), minContrast, "%s %s on %v", theme.name, name, bg)
			}
		}
		fills := map[string][2]color.Color{
			"AccentFill":    {p.OnAccent, p.AccentFill},
			"SecondaryFill": {p.OnAccent, p.SecondaryFill},
			"SuccessFill":   {p.OnAccent, p.SuccessFill},
			"DangerFill":    {p.OnAccent, p.DangerFill},
			"InfoFill":      {p.OnAccent, p.InfoFill},
			"Warn":          {p.OnWarn, p.Warn},
		}
		for name, pair := range fills {
			assert.GreaterOrEqual(t, contrast(pair[0], pair[1]), minContrast, "%s text on %s", theme.name, name)
		}
	}
}

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

	m.requestConfirm(confirmOverlay{act: mutation.DrainNode, target: "n1", run: func() tea.Cmd { return nil }})
	assert.Contains(t, m.confirmView(), "\x1b[1;7m", "the focused button is bold reverse")

	_, _ = m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	assert.False(t, m.styles.Selected.GetReverse(), "color comes back")
}
