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
		assert.True(t, m.confirm.onAction)
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
		assert.False(t, m.confirm.onAction)
	})
	t.Run("enter on an irreversible action only nudges", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Irreversible)
		_, _ = m.updateConfirmView(keyPress("right"))
		_, _ = m.updateConfirmView(keyPress("enter"))
		assert.False(t, *ran, "enter must never confirm an irreversible action")
		assert.Equal(t, common.ConfirmView, m.viewMode)
		assert.True(t, m.confirm.nudge)
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
	assert.Equal(t, "1j", m.rowPicker.filter)
	assert.Empty(t, m.rowPicker.visible())
	assert.Contains(t, ansi.Strip(m.rowPickerView()), "no matches")
	_, _ = m.updateRowPickerView(keyPress("enter"))
	assert.Equal(t, common.RowPickerView, m.viewMode, "enter on no match is a no-op")

	_, _ = m.updateRowPickerView(keyPress("backspace"))
	assert.Equal(t, "1", m.rowPicker.filter)
	require.Len(t, m.rowPicker.visible(), 3) // row-01, row-10, row-11
	_, _ = m.updateRowPickerView(keyPress("down"))
	_, _ = m.updateRowPickerView(keyPress("enter"))
	assert.Equal(t, "row-10", chosen.Name)
}

func TestRowPicker_ShortListKeepsJK(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.openRowPicker("Pick", "go", pickerTargets(3), func(domain.Scope) tea.Cmd { return nil })
	_, _ = m.updateRowPickerView(keyPress("j"))
	assert.Equal(t, 1, m.rowPicker.cursor)
	assert.Empty(t, m.rowPicker.filter)
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
		want[b.Help().Desc] = false
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
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "four", m.toasts.active.msg)
	require.Len(t, m.toasts.older, 2, "at most three show")
	assert.Equal(t, "two", m.toasts.older[0].msg)

	stacked := ansi.Strip(m.renderToasts())
	assert.Less(t, strings.Index(stacked, "two"), strings.Index(stacked, "four"), "newest at the bottom")

	m.handleToastExpireMsg(toastExpireMsg{id: m.toasts.older[0].id}) // "two"
	assert.Len(t, m.toasts.older, 1)
	m.handleToastExpireMsg(toastExpireMsg{id: m.toasts.active.id}) // "four"
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "three", m.toasts.active.msg, "the next older takes its place")
	assert.Empty(t, m.toasts.older)
}
