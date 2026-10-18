package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// Below the width threshold v says why and leaves the layout alone.
func TestPreview_NeedsWidth(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	tableW := m.table.Width()
	_, _ = m.Update(keyPress("v"))
	assert.False(t, m.preview)
	require.NotNil(t, m.toasts.active)
	assert.Contains(t, m.toasts.active.msg, "≥160")
	assert.Equal(t, tableW, m.table.Width())
}

// On a wide terminal v splits the panel 60/40 and back.
func TestPreview_TogglesSplit(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	full := m.table.Width()
	_, _ = m.Update(keyPress("v"))
	require.True(t, m.previewShown())
	assert.Equal(t, 200-m.previewOuterWidth()-2, m.table.Width())

	lines := strings.Split(ansi.Strip(m.render()), "\n")
	assert.Len(t, lines, 40)
	for i, l := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(l), 200, "line %d", i)
	}
	assert.Contains(t, lines[2], "╭─ dac-alpha ─", "the pane is titled with the selected row")

	_, _ = m.Update(keyPress("v"))
	assert.False(t, m.previewShown())
	assert.Equal(t, full, m.table.Width())
}

// The pane follows the cursor.
func TestPreview_FollowsCursor(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	_, _ = m.Update(keyPress("v"))
	first := ansi.Strip(m.previewBody(60))
	assert.Contains(t, first, "dac-alpha")
	_, _ = m.Update(keyPress("down"))
	second := ansi.Strip(m.previewBody(60))
	assert.NotEqual(t, first, second)
	assert.Contains(t, second, m.selectedRawRow()[0])
}

// Narrowing the terminal hides the pane; widening brings it back.
func TestPreview_HidesWhenNarrowed(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	_, _ = m.Update(keyPress("v"))
	m.updateLayout(120, 40)
	assert.False(t, m.previewShown())
	assert.True(t, m.preview, "still on for the session")
	m.updateLayout(200, 40)
	assert.True(t, m.previewShown())
}

func TestPalette_PreviewToggle(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	typePalette(m, "preview pane")
	_, _ = m.Update(keyPress("enter"))
	assert.True(t, m.previewShown())
}
