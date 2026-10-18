package tui

import (
	"image/color"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Contains(t, m.renderToast(40), "38;2;215;0;0", "Danger border")
}

// A light-background reply restyles the bubbles, not just the theme.
func TestBackgroundColorMsg_RestylesComponents(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	light := NewPalette(false)
	require.Equal(t, light, m.theme.Palette)
	assert.Equal(t, light.Accent, m.loadingSpinner.Style.GetForeground())
	assert.Equal(t, light.Info, m.help.Styles.FullKey.GetForeground())
	assert.Equal(t, light.Accent, m.styles.Selected.GetBackground())
}
