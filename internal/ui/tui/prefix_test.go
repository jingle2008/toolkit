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
	assert.Empty(t, m.prefix)
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
	_, _ = m.Update(keyPress("s"))
	require.Equal(t, "s", m.prefix)
	_, _ = m.Update(keyPress("esc"))
	assert.Empty(t, m.prefix)
	assert.Equal(t, "alpha", m.filter)
}

// Typing in the filter never starts a prefix.
func TestPrefix_FilterInputBypasses(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	_, _ = m.Update(keyPress("/"))
	require.Equal(t, common.EditInput, m.inputMode)
	_, _ = m.Update(keyPress("s"))
	assert.Empty(t, m.prefix)
	assert.Equal(t, "s", m.textInput.Value())
}

func TestPrefix_StatusMarker(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	_, _ = m.Update(keyPress("s"))
	assert.Contains(t, ansi.Strip(m.statusView()), "s…")
	_, _ = m.Update(keyPress("n"))
	assert.NotContains(t, ansi.Strip(m.statusView()), "s…")
}

// The headers show the sort keys before s is pressed: each sortable
// column's key letter is underlined, pending prefix or not.
func TestPrefix_SortUnderlinesHeaderKeys(t *testing.T) {
	t.Parallel()
	m := newPrefixModel(t)
	want := []string{"\x1b[4mN\x1b[24mame", "\x1b[4mT\x1b[24menant", "\x1b[4mU\x1b[24msage"}
	for _, step := range []string{"", "s", "esc"} {
		if step != "" {
			_, _ = m.Update(keyPress(step))
		}
		header := m.table.View()
		for _, w := range want {
			assert.Contains(t, header, w, "after %q", step)
		}
	}
}

func TestUnderlineSortKey(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "\x1b[4mN\x1b[24mame", underlineSortKey("Name", "n"))
	assert.Equal(t, "S\x1b[4mi\x1b[24mze", underlineSortKey("Size", "i"))
	assert.Equal(t, "Age", underlineSortKey("Age", "z"))
}

// A sort key no column has cancels the prefix and says which keys would
// have worked, in column order.
func TestSortPrefix_UnknownKeyToastsTheKeys(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.Tenant)
	_, _ = m.Update(keyPress("s"))
	_, _ = m.Update(keyPress("x"))
	assert.Empty(t, m.prefix)
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "no sort on x · try s n/i/s", m.toasts.active.msg)
}
