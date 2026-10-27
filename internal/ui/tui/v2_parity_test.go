package tui

import (
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// bubbles v2 viewports bind h/l to horizontal scroll; ours leave them to
// the app so h stays Help.
func TestViewports_NoHorizontalScrollKeys(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	logVP := m.log.Viewport()
	for name, vp := range map[string]bool{
		"details": m.viewport.KeyMap.Left.Enabled() || m.viewport.KeyMap.Right.Enabled(),
		"log":     logVP.KeyMap.Left.Enabled() || logVP.KeyMap.Right.Enabled(),
	} {
		assert.False(t, vp, "%s viewport still scrolls sideways", name)
	}
}

func TestDetailsView_HOpensHelp(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotNil(t, m.enterDetailView())
	_, _ = m.Update(keyPress("h"))
	assert.Equal(t, common.HelpView, m.viewMode)
}

// The model asks for the terminal background and restyles on the reply.
func TestBackgroundColorMsg_Restyles(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	dark := m.theme.HelpKey.GetForeground()
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	assert.NotEqual(t, dark, m.theme.HelpKey.GetForeground(), "light background keeps dark-theme colors")
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.Black})
	assert.Equal(t, dark, m.theme.HelpKey.GetForeground())
}
