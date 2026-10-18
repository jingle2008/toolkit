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
	require.NotEmpty(t, m.palette.matches)
	assert.Equal(t, "DedicatedAICluster", m.palette.items[m.palette.matches[0].Index].label,
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
	require.NotEmpty(t, p.matches)
	assert.Equal(t, "GPUNode", p.items[p.matches[0].Index].label)
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
	assert.Empty(t, m.palette.matches)
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
	require.NotEmpty(t, m.palette.matches)
	assert.Equal(t, "Sort by Tenant", m.palette.items[m.palette.matches[0].Index].label)
	_, _ = m.Update(keyPress("enter"))
	assert.Equal(t, "Tenant", m.sortColumn)

	typePalette(m, "faulty only")
	_, _ = m.Update(keyPress("enter"))
	assert.True(t, m.showFaulty)
}

// Environments are offered where quick switch works, and switch on enter.
func TestPalette_SwitchesEnvironment(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.updateLayout(120, 40)
	typePalette(m, envORD.KubeContext())
	require.NotEmpty(t, m.palette.matches)
	it := m.palette.items[m.palette.matches[0].Index]
	require.Equal(t, "env", it.kind)
	_, _ = m.Update(keyPress("enter"))
	assert.True(t, m.environment.Equals(envORD))
}

func TestPalette_NoEnvironmentsOutsideScopedCategories(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	m.category = domain.Environment
	require.False(t, m.category.IsEnvironmentScoped())
	m.openPalette()
	for _, it := range m.palette.items {
		assert.NotEqual(t, "env", it.kind, it.label)
	}
}

// The list scrolls under the cursor and never outgrows the screen.
func TestPalette_Scrolls(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 20)
	showCategory(t, m, domain.DedicatedAICluster)
	m.openPalette()
	n := m.paletteRows()
	require.Greater(t, len(m.palette.matches), n)
	for range len(m.palette.matches) + 3 {
		_, _ = m.Update(keyPress("down"))
	}
	last := m.palette.items[m.palette.matches[len(m.palette.matches)-1].Index].label
	view := ansi.Strip(m.paletteView())
	assert.Contains(t, lineWith(view, last), "▸")
	assert.Contains(t, view, fmt.Sprintf("of %d", len(m.palette.matches)))
	assert.LessOrEqual(t, strings.Count(ansi.Strip(m.render()), "\n")+1, 20)
}

// A match never strings letters across label and hint: "dac" must not
// match an environment as d(p-prod) a(p-…) c(huncheon).
func TestPalette_NoCrossFieldMatches(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	m.openPalette()
	m.palette.items = []paletteItem{
		{kind: "env", label: "dp-prod-yny", hint: "oc1 ap-chuncheon-1"},
		{kind: "category", label: "DedicatedAICluster", hint: "dac"},
	}
	m.palette.input.SetValue("dac")
	m.refilterPalette()
	require.Len(t, m.palette.matches, 1)
	assert.Equal(t, "DedicatedAICluster", m.palette.items[m.palette.matches[0].Index].label)
}
