package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/internal/ui/tui/toast"
)

func TestConfirmButtons_FocusAndEnter(t *testing.T) {
	t.Parallel()
	t.Run("enter on the default Cancel cancels", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("enter"))
		assert.False(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("→ then enter confirms a recoverable action", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("right"))
		assert.True(t, m.confirm.OnAction())
		_, _ = m.updateConfirmView(keyPress("enter"))
		assert.True(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("tab toggles focus back", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("tab"))
		_, _ = m.updateConfirmView(keyPress("tab"))
		assert.False(t, m.confirm.OnAction())
	})
	t.Run("enter on an irreversible action only nudges", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Irreversible)
		_, _ = m.updateConfirmView(keyPress("right"))
		_, _ = m.updateConfirmView(keyPress("enter"))
		assert.False(t, *ran, "enter must never confirm an irreversible action")
		assert.Equal(t, common.ConfirmView, m.viewMode)
		assert.True(t, m.confirm.Nudged())
		assert.Contains(t, ansi.Strip(m.confirmView()), "press capital Y")
		_, _ = m.updateConfirmView(keyPress("Y"))
		assert.True(t, *ran)
	})
}

// pickerTargets returns n tenant targets named row-00, row-01, ...
func pickerTargets(n int) []domain.Scope {
	out := make([]domain.Scope, n)
	for i := range out {
		out[i] = domain.Scope{Category: domain.Tenant, Name: fmt.Sprintf("row-%02d", i)}
	}
	return out
}

func TestRowPicker_FilterPastEightRows(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	var chosen domain.Scope
	m.openRowPicker("Pick", "go", pickerTargets(12), func(s domain.Scope) tea.Cmd { chosen = s; return nil })

	for _, k := range []string{"1", "j"} { // j types: the filter owns letters
		_, _ = m.updateRowPickerView(keyPress(k))
	}
	assert.Equal(t, "1j", m.rowPicker.Filter())
	assert.Empty(t, m.rowPicker.Visible())
	assert.Contains(t, ansi.Strip(m.rowPickerView()), "no matches")
	_, _ = m.updateRowPickerView(keyPress("enter"))
	assert.Equal(t, common.RowPickerView, m.viewMode, "enter on no match is a no-op")

	_, _ = m.updateRowPickerView(keyPress("backspace"))
	assert.Equal(t, "1", m.rowPicker.Filter())
	require.Len(t, m.rowPicker.Visible(), 3) // row-01, row-10, row-11
	_, _ = m.updateRowPickerView(keyPress("down"))
	_, _ = m.updateRowPickerView(keyPress("enter"))
	assert.Equal(t, "row-10", chosen.Name)
}

func TestRowPicker_ShortListKeepsJK(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.openRowPicker("Pick", "go", pickerTargets(3), func(domain.Scope) tea.Cmd { return nil })
	_, _ = m.updateRowPickerView(keyPress("j"))
	assert.Equal(t, 1, m.rowPicker.Cursor())
	assert.Empty(t, m.rowPicker.Filter())
	assert.NotContains(t, ansi.Strip(m.rowPickerView()), "filter:")
}

// The visible window follows the cursor and says where it is.
func TestRowPicker_Scrolls(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.updateLayout(80, 15) // modal body: 15*9/10-4 = 9 rows
	m.openRowPicker("Pick", "go", pickerTargets(30), func(domain.Scope) tea.Cmd { return nil })
	n := m.pickerRows()
	require.Less(t, n, 30)
	for range 29 {
		_, _ = m.updateRowPickerView(keyPress("down"))
	}
	view := ansi.Strip(m.rowPickerView())
	assert.Contains(t, lineWith(view, "row-29"), "▸")
	assert.NotContains(t, view, "row-00")
	assert.Contains(t, view, fmt.Sprintf("%d–30 of 30", 30-n+1))
	assert.LessOrEqual(t, strings.Count(view, "\n")+1, 15, "picker fits the screen")
}

// On a short terminal help scrolls instead of truncating, and every
// binding can be scrolled into view.
func TestHelp_ScrollReachesEveryBinding(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 15)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()

	want := map[string]bool{}
	for _, b := range m.keys.Context {
		label := b.Help().Desc
		if col, ok := keys.SortColumnOf(b); ok {
			label = col // the Sort section lists columns
		}
		want[label] = false
	}
	for _, b := range m.keys.Global {
		want[b.Help().Desc] = false
	}
	_, maxScroll := m.helpWindow(len(m.helpLines()))
	require.Positive(t, maxScroll, "help must overflow 80x15 for this test")
	for range maxScroll + 2 { // past the end: scrolling clamps
		view := ansi.Strip(m.fullHelpView())
		assert.LessOrEqual(t, strings.Count(view, "\n")+1, 15)
		for d := range want {
			if strings.Contains(view, d) {
				want[d] = true
			}
		}
		_, _ = m.updateHelpView(keyPress("j"))
	}
	assert.Equal(t, maxScroll, m.helpScroll)
	for d, seen := range want {
		assert.True(t, seen, "binding %q never shown", d)
	}
	_, _ = m.updateHelpView(keyPress("home"))
	assert.Zero(t, m.helpScroll)
}

func TestToasts_StackAndExpire(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, s := range []string{"one", "two", "three", "four"} {
		_ = m.showToast(s, toastInfo)
	}
	require.NotNil(t, m.toasts.Active())
	assert.Equal(t, "four", m.toasts.Active().Msg)
	require.Len(t, m.toasts.Older(), 2, "at most three show")
	assert.Equal(t, "two", m.toasts.Older()[0].Msg)

	stacked := ansi.Strip(m.renderToasts())
	assert.Less(t, strings.Index(stacked, "two"), strings.Index(stacked, "four"), "newest at the bottom")

	m.toasts, _ = m.toasts.Update(toast.ExpireMsg{ID: m.toasts.Older()[0].ID}) // "two"
	assert.Len(t, m.toasts.Older(), 1)
	m.toasts, _ = m.toasts.Update(toast.ExpireMsg{ID: m.toasts.Active().ID}) // "four"
	require.NotNil(t, m.toasts.Active())
	assert.Equal(t, "three", m.toasts.Active().Msg, "the next older takes its place")
	assert.Empty(t, m.toasts.Older())
}

// A filterable picker says typing filters.
func TestRowPicker_HintSaysTypeToFilter(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	m.openRowPicker("Go to", "go", pickerTargets(3), nil)
	assert.NotContains(t, ansi.Strip(m.rowPickerView()), "type to filter")
	m.openRowPicker("Go to", "go", pickerTargets(12), nil)
	assert.Contains(t, ansi.Strip(m.rowPickerView()), "type to filter")
}

// / filters help by key or description; a group whose title matches is
// kept whole; enter keeps the filter and esc clears it before closing.
func TestHelp_Filter(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()

	m.updateHelpView(keyPress("/"))
	require.True(t, m.helpTyping)
	for _, r := range "drain" {
		m.updateHelpView(keyPress(string(r)))
	}
	view := ansi.Strip(m.fullHelpView())
	assert.Contains(t, view, "Drain")
	assert.NotContains(t, view, "Refresh")
	assert.NotContains(t, view, "General", "groups with no match are left out")
	assert.Contains(t, view, "/drain")

	m.updateHelpView(keyPress("enter"))
	assert.False(t, m.helpTyping)
	assert.Contains(t, ansi.Strip(m.fullHelpView()), "⟨/drain⟩")

	m.updateHelpView(keyPress("esc"))
	assert.Empty(t, m.helpFilter)
	assert.Equal(t, common.HelpView, m.viewMode, "the first esc clears the filter")
	m.updateHelpView(keyPress("esc"))
	assert.Equal(t, common.ListView, m.viewMode)
}

func TestHelp_FilterKeepsMatchingGroupAndSaysNone(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()
	m.helpFilter = "sort"
	view := ansi.Strip(m.fullHelpView())
	assert.Contains(t, view, "s f         Free", "the Sort group's entries are labeled by column")

	m.helpFilter = "zzz"
	assert.Contains(t, ansi.Strip(m.fullHelpView()), "no keys match · esc clears the filter")
}

// h and ? are text while typing a help filter, not close keys.
func TestHelp_FilterTakesKeysAsText(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()
	m.updateHelpView(keyPress("/"))
	m.updateHelpView(keyPress("h"))
	m.updateHelpView(keyPress("?"))
	assert.Equal(t, "h?", m.helpFilter)
	assert.Equal(t, common.HelpView, m.viewMode)
}

// A modal nearly as tall as the screen draws on a blank screen, not over
// fragments of the view; a smaller one keeps the dimmed view around it.
func TestOverlayModal_BlankBehindFullHeight(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 15)
	showCategory(t, m, domain.GPUNode)
	tall := strings.TrimSuffix(strings.Repeat("x\n", 13), "\n")
	out := ansi.Strip(m.overlayModal(common.ListView, tall))
	assert.NotContains(t, out, "GPUNode")
	assert.Len(t, strings.Split(out, "\n"), 15, "still a full screen")

	short := ansi.Strip(m.overlayModal(common.ListView, "x"))
	assert.Contains(t, short, "GPUNode")
}
