package tui

import (
	"image/color"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// Every role has a color on both backgrounds.
func TestNewPalette_AllRolesSet(t *testing.T) {
	t.Parallel()
	for _, dark := range []bool{true, false} {
		p := reflect.ValueOf(NewPalette(dark))
		for i := range p.NumField() {
			assert.NotNil(t, p.Field(i).Interface(), "dark=%v %s", dark, p.Type().Field(i).Name)
		}
	}
}

// The error toast is drawn in the Danger role (#D70000 on dark), which the
// ANSI-stripped goldens can't see.
func TestRenderToast_ErrorUsesDanger(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.showToast("boom", toastError)
	assert.Contains(t, m.renderToast(40), "38;2;255;128;128", "Danger border")
}

// A light-background reply restyles the bubbles, not just the theme.
func TestBackgroundColorMsg_RestylesComponents(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	light := NewPalette(false)
	require.Equal(t, light, m.theme.Palette)
	assert.Equal(t, light.Accent, m.loadingSpinner.Style.GetForeground())
	assert.Equal(t, light.Selection, m.styles.Selected.GetBackground())
	r, ok := m.renderer.(view.ProductionRenderer)
	require.True(t, ok)
	assert.True(t, r.Light, "the JSON view follows the background")
}

// With both of the terminal's colors reported, the neutrals come from
// them; with only the background, the fixed palette for it.
func TestRestyle_DerivesFromTerminalColors(t *testing.T) {
	t.Parallel()
	fg, bg := lipgloss.Color("#CBE0F0"), lipgloss.Color("#011423")
	m := newGoldenModel(t, 120, 30)
	_, _ = m.Update(tea.BackgroundColorMsg{Color: bg})
	assert.Equal(t, NewPalette(true), m.theme.Palette, "background alone: the fixed dark palette")
	_, _ = m.Update(tea.ForegroundColorMsg{Color: fg})
	assert.Equal(t, DerivePalette(true, fg, bg), m.theme.Palette)
	assert.Equal(t, fg, m.theme.Palette.Text)
	assert.Equal(t, NewPalette(true).Accent, m.theme.Palette.Accent, "hues stay fixed")
}
