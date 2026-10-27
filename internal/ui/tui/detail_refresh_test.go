package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/toast"
	"github.com/jingle2008/toolkit/pkg/models"
)

// applyRenders runs cmd and feeds every details render it produces back to
// m, reporting whether there was one.
func applyRenders(m *Model, cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		rendered := false
		for _, c := range msg {
			rendered = applyRenders(m, c) || rendered
		}
		return rendered
	case detailContentRenderedMsg:
		m.Update(msg)
		return true
	}
	return false
}

// reloadDACs delivers a DAC reload, as the live watch does.
func reloadDACs(m *Model, items map[string][]models.DedicatedAICluster) tea.Cmd {
	_, cmd := m.Update(categoryLoadedMsg{Cat: domain.DedicatedAICluster, Gen: m.gens.msg, Rows: dacRows(items)})
	return cmd
}

// Details follows a reload of its item and keeps its scroll position.
func TestDetails_RefreshesOnReload(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 14) // short enough that details scrolls
	showDetails(t, m)
	m.viewport.SetYOffset(2)
	require.Equal(t, 2, m.viewport.YOffset())

	dac := m.dataset.DedicatedAIClusterMap["acme"][0]
	dac.Status = "Updating"
	require.True(t, applyRenders(m, reloadDACs(m, map[string][]models.DedicatedAICluster{"acme": {dac}})))
	assert.Contains(t, m.viewport.GetContent(), "Updating")
	assert.Equal(t, 2, m.viewport.YOffset(), "the scroll position is kept")
}

// An item deleted while details is open stays on screen, marked gone; it
// still switches layout.
func TestDetails_DeletedItemMarkedGone(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showDetails(t, m)

	require.True(t, applyRenders(m, reloadDACs(m, map[string][]models.DedicatedAICluster{})))
	out := ansi.Strip(m.render())
	assert.Contains(t, out, "acme/dac-alpha · gone")
	assert.Contains(t, m.viewport.GetContent(), "A100_80G", "the last content stays")

	_, cmd := m.Update(keyPress("tab"))
	require.True(t, applyRenders(m, cmd))
	assert.Contains(t, ansi.Strip(m.viewport.GetContent()), `"unitShape"`, "JSON of the gone item")
}

// Messages that don't rebuild the rows don't re-render details.
func TestDetails_NoRenderWithoutReload(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showDetails(t, m)
	gen := m.gens.detail
	m.Update(toast.ExpireMsg{ID: 99})
	assert.Equal(t, gen, m.gens.detail)
}
