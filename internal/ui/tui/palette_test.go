package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/palette"
)

// typePalette opens the palette and types q into it.
func typePalette(m *Model, q string) {
	_, _ = m.Update(keyPress(":"))
	for _, r := range q {
		_, _ = m.Update(keyPress(string(r)))
	}
}

// An exact alias wins on enter, whatever the ranking: :dac⏎ opens
// DedicatedAICluster as it always has.
func TestPalette_ExactAliasOpensCategory(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	showCategory(t, m, domain.Tenant)
	typePalette(m, "dac")
	require.Equal(t, common.PaletteView, m.viewMode)
	require.NotEmpty(t, m.palette.Matches())
	assert.Equal(t, "DedicatedAICluster", m.palette.Matches()[0].Label,
		"the alias's category is listed first, as enter runs it")
	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.DedicatedAICluster, m.category)
}

func TestPalette_FuzzyRanksAndHighlights(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	typePalette(m, "gpunode")
	p := m.palette
	require.NotEmpty(t, p.Matches())
	assert.Equal(t, "GPUNode", p.Matches()[0].Label)
	view := m.paletteView()
	assert.Contains(t, ansi.Strip(view), "▸ GPUNode")
	assert.Contains(t, view, "\x1b[1;", "matched characters are highlighted")

	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, domain.GPUNode, m.category)
}

func TestPalette_NoMatches(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	typePalette(m, "zzzzqqq")
	assert.Empty(t, m.palette.Matches())
	assert.Contains(t, ansi.Strip(m.paletteView()), "no matches")
	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, common.PaletteView, m.viewMode, "nothing to run: stay open to edit the query")
	assert.Equal(t, domain.DedicatedAICluster, m.category)
}

func TestPalette_EscCloses(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	typePalette(m, "gpu")
	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.DedicatedAICluster, m.category)
}

// Actions run as their keys would: a sort, and the faulty toggle.
func TestPalette_RunsActions(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	typePalette(m, "sort by tenant")
	require.NotEmpty(t, m.palette.Matches())
	assert.Equal(t, "Sort by Tenant", m.palette.Matches()[0].Label)
	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, "Tenant", m.sortColumn)

	typePalette(m, "faulty only")
	_, _ = m.Update(keyPress("enter"))
	assert.True(t, m.showFaulty)
}

// Sort by Name, which every list view has, is offered too, with its key.
func TestPalette_OffersSortByName(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	m.sortColumn, m.sortAsc = "Tenant", true
	typePalette(m, "sort by name")
	require.NotEmpty(t, m.palette.Matches())
	it := m.palette.Matches()[0]
	assert.Equal(t, "Sort by Name", it.Label)
	assert.Equal(t, "s n", it.Hint)
	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, common.NameCol, m.sortColumn)
}

// Environments are offered where quick switch works, and switch on enter.
func TestPalette_SwitchesEnvironment(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.updateLayout(120, 40)
	typePalette(m, envORD.KubeContext())
	require.NotEmpty(t, m.palette.Matches())
	it := m.palette.Matches()[0]
	require.Equal(t, "env", it.Kind)
	_, _ = m.Update(keyPress("enter"))
	assert.True(t, m.environment.Equals(envORD))
}

func TestPalette_NoEnvironmentsOutsideScopedCategories(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	m.category = domain.Environment
	require.False(t, m.category.IsEnvironmentScoped())
	m.openPalette()
	for _, it := range m.palette.Items() {
		assert.NotEqual(t, "env", it.Kind, it.Label)
	}
}

// The list scrolls under the cursor and never outgrows the screen.
func TestPalette_Scrolls(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 20)
	showCategory(t, m, domain.DedicatedAICluster)
	m.openPalette()
	n := m.paletteRows()
	require.Greater(t, len(m.palette.Matches()), n)
	for range len(m.palette.Matches()) + 3 {
		_, _ = m.Update(keyPress("down"))
	}
	last := m.palette.Matches()[len(m.palette.Matches())-1].Label
	view := ansi.Strip(m.paletteView())
	assert.Contains(t, lineWith(view, last), "▸")
	assert.Contains(t, view, fmt.Sprintf("of %d", len(m.palette.Matches())))
	assert.LessOrEqual(t, strings.Count(ansi.Strip(m.render()), "\n")+1, 20)
}

// Category hints are the short aliases only, and the open category is
// marked.
func TestPalette_CategoryHints(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t) // DedicatedAICluster open
	assert.Equal(t, "gpup gp", m.categoryHint(domain.GPUPool, domain.GPUPool.Aliases()))
	assert.True(t, strings.HasSuffix(m.categoryHint(domain.DedicatedAICluster, domain.DedicatedAICluster.Aliases()), currentSlotMarker))
	assert.NotContains(t, m.categoryHint(domain.GPUPool, domain.GPUPool.Aliases()), "gpupool")
}

// With no query, the recently run items come first, newest first, under
// "recent"; each group is named on its first row only.
func TestPalette_RecentFirst(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	typePalette(m, "gpunode")
	_, _ = m.Update(keyPress("enter"))
	require.Equal(t, domain.GPUNode, m.category)
	typePalette(m, "tenant")
	_, _ = m.Update(keyPress("enter"))
	require.Equal(t, domain.Tenant, m.category)

	_, _ = m.Update(keyPress(":"))
	p := m.palette
	require.Equal(t, 2, p.Recent())
	assert.Equal(t, "Tenant", p.Matches()[0].Label)
	assert.Equal(t, "GPUNode", p.Matches()[1].Label)

	lines := strings.Split(ansi.Strip(m.paletteView()), "\n")
	assert.Equal(t, 1, strings.Count(strings.Join(lines, "\n"), "recent"), "named once")
	assert.Contains(t, lineWith(strings.Join(lines, "\n"), "Tenant"), "recent")
	assert.Equal(t, 1, strings.Count(strings.Join(lines, "\n"), " category"), "next group named once")
}

// The recent list keeps palette.RecentMax items without repeats.
func TestPalette_RememberCapsAndDedupes(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	for i := range palette.RecentMax + 2 {
		m.rememberPalette(palette.Item{Kind: "action", Label: fmt.Sprint(i)})
	}
	m.rememberPalette(palette.Item{Kind: "action", Label: "3"})
	assert.Len(t, m.paletteRecent, palette.RecentMax)
	assert.Equal(t, palette.Item{Kind: "action", Label: "3"}.ID(), m.paletteRecent[0])
	assert.Equal(t, 1, strings.Count(strings.Join(m.paletteRecent, "|"), "\x003"))
}
