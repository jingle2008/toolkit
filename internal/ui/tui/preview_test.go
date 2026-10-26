package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Below the width threshold v says why and leaves the layout alone.
func TestPreview_NeedsWidth(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	tableW := m.table.Width()
	_, _ = m.Update(keyPress("v"))
	assert.False(t, m.preview)
	require.NotNil(t, m.toasts.Active())
	assert.Contains(t, m.toasts.Active().Msg, "≥160")
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

// Frames reuse the body until the dataset, the selection, the width or
// the theme changes.
func TestPreview_CachesBody(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	_, _ = m.Update(keyPress("v"))
	w := m.previewOuterWidth() - 4 // as previewPanel asks
	first := m.previewBody(w)
	require.Contains(t, ansi.Strip(first), "dac-alpha")

	// A sentinel in the cache shows a frame read it rather than rebuilt.
	const sentinel = "cached"
	m.previewMemo.body = sentinel
	assert.Equal(t, sentinel, m.previewBody(w), "the next frame reuses the body")
	m.render()
	assert.Equal(t, sentinel, m.previewBody(w), "a whole frame doesn't rebuild it")

	assert.NotEqual(t, sentinel, m.previewBody(w+1), "a width change rebuilds it")
	assert.Equal(t, first, m.previewBody(w))
	m.previewMemo.body = sentinel

	// A load replaces the dataset: the next frame shows the new item.
	dacs := cloneGroups(m.dataset.DedicatedAIClusterMap)
	for k, group := range dacs {
		for i := range group {
			if group[i].Name == "dac-alpha" {
				group[i].Profile = "reloaded-profile"
			}
		}
		dacs[k] = group
	}
	m.applyDataset(func(ds *models.Dataset) { ds.DedicatedAIClusterMap = dacs }, domain.DedicatedAICluster, 0)
	reloaded := ansi.Strip(m.previewBody(w))
	assert.NotEqual(t, sentinel, reloaded, "a dataset change rebuilds it")
	assert.Contains(t, reloaded, "reloaded-profile")

	m.previewMemo.body = sentinel
	_, _ = m.Update(keyPress("down"))
	assert.NotEqual(t, sentinel, m.previewBody(w), "a selection change rebuilds it")

	m.previewMemo.body = sentinel
	m.restyle()
	assert.NotEqual(t, sentinel, m.previewBody(w), "a theme change rebuilds it")
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
