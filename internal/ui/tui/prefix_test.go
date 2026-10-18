package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// newPrefixModel is the golden DAC list: sortable Name, Tenant, Internal,
// Usage, Size and Age.
func newPrefixModel(t *testing.T) *Model {
	t.Helper()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	return m
}

func TestPrefix_SortByColumnKey(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	for k, col := range map[string]string{"t": "Tenant", "i": "Internal", "u": "Usage", "a": "Age", "n": "Name"} {
		_, _ = m.Update(keyPress("s"))
		_, _ = m.Update(keyPress(k))
		assert.Equal(t, col, m.sortColumn, "s %s", k)
	}
	sortBy := func(k string) {
		_, _ = m.Update(keyPress("s"))
		_, _ = m.Update(keyPress(k))
	}
	sortBy("t")
	sortBy("n")
	require.True(t, m.sortAsc, "a new sort column starts ascending")
	sortBy("n")
	assert.False(t, m.sortAsc, "repeating a sort flips its direction")
}

// A second key that completes nothing cancels the prefix and is not
// replayed: s then q must not quit.
func TestPrefix_UnknownContinuationCancels(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	_, _ = m.Update(keyPress("s"))
	_, cmd := m.Update(keyPress("q"))
	assert.Empty(t, m.prefix.key)
	if cmd != nil {
		_, quit := cmd().(tea.QuitMsg)
		assert.False(t, quit, "s q must not quit")
	}
}

// esc with a prefix pending cancels only the prefix; the filter stays.
func TestPrefix_EscCancelsOnlyThePrefix(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	m.filter = "alpha"
	_, _ = m.Update(keyPress("g"))
	require.Equal(t, "g", m.prefix.key)
	_, _ = m.Update(keyPress("esc"))
	assert.Empty(t, m.prefix.key)
	assert.Equal(t, "alpha", m.filter)
}

// Typing in the filter never starts a prefix.
func TestPrefix_FilterInputBypasses(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	_, _ = m.Update(keyPress("/"))
	require.Equal(t, common.EditInput, m.inputMode)
	_, _ = m.Update(keyPress("s"))
	assert.Empty(t, m.prefix.key)
	assert.Equal(t, "s", m.textInput.Value())
}

func TestPrefix_WhichKeyAfterTimeout(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	_, cmd := m.Update(keyPress("g"))
	require.NotNil(t, cmd, "a prefix schedules its timeout")
	assert.Contains(t, ansi.Strip(m.statusView()), "g…")

	_, _ = m.Update(prefixTimeoutMsg{gen: m.prefix.gen - 1})
	assert.False(t, m.prefix.whichKey, "a stale timeout is ignored")
	_, _ = m.Update(prefixTimeoutMsg{gen: m.prefix.gen})
	assert.True(t, m.prefix.whichKey)
	assert.Contains(t, ansi.Strip(m.render()), "g  top")

	_, _ = m.Update(keyPress("g"))
	assert.False(t, m.prefix.whichKey)
	assert.NotContains(t, ansi.Strip(m.render()), "g  top")
}

// The sort prefix shows its keys in the headers, not a popup: each
// sortable column's key letter is underlined while s is pending.
func TestPrefix_SortUnderlinesHeaderKeys(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	assert.NotContains(t, m.table.View(), "\x1b[4mT\x1b[24m")
	_, _ = m.Update(keyPress("s"))
	_, _ = m.Update(prefixTimeoutMsg{gen: m.prefix.gen})
	assert.False(t, m.prefix.whichKey, "no popup for s")
	header := m.table.View()
	for _, want := range []string{"\x1b[4mN\x1b[24mame", "\x1b[4mT\x1b[24menant", "\x1b[4mU\x1b[24msage"} {
		assert.Contains(t, header, want)
	}
	_, _ = m.Update(keyPress("esc"))
	assert.NotContains(t, m.table.View(), "\x1b[4m", "underlines go with the prefix")
}

func TestHelp_GotoEnds(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 15)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()
	_, maxScroll := m.helpWindow(len(m.helpLines()))
	_, _ = m.updateHelpView(keyPress("G"))
	assert.Equal(t, maxScroll, m.helpScroll)
	_, _ = m.updateHelpView(keyPress("g"))
	_, _ = m.updateHelpView(keyPress("g"))
	assert.Zero(t, m.helpScroll)

	// esc with g pending cancels the prefix and leaves help open.
	_, _ = m.updateHelpView(keyPress("g"))
	_, _ = m.updateHelpView(keyPress("esc"))
	assert.Equal(t, common.HelpView, m.viewMode)
	assert.Empty(t, m.prefix.key)
}

func TestUnderlineSortKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "\x1b[4mN\x1b[24mame", underlineSortKey("Name", "n"))
	assert.Equal(t, "S\x1b[4mi\x1b[24mze", underlineSortKey("Size", "i"))
	assert.Equal(t, "Age", underlineSortKey("Age", "z"))
}
