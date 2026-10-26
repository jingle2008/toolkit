// Package tui — the modal row picker.
//
// The picker lives in the rowpicker sub-package; this file is the
// parent's glue: opening it with what choosing does (a closure over the
// live Model, invoked synchronously on the update goroutine, never inside
// a tea.Cmd), the quit key, dispatching a rowpicker.Result (run the
// choice, or just close), restoring the view it was opened from, and the
// theme-derived styles and modal size it draws in.
package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/internal/ui/tui/rowpicker"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// rowPickerPending is the parent's half of the open row picker: what
// choosing a row does and the view to restore on close.
type rowPickerPending struct {
	choose     func(domain.Scope) tea.Cmd
	returnView common.ViewMode
}

// openRowPicker shows the row picker over targets; choose runs on enter.
//
//nolint:unparam // returns nil so callers can `return m.openRowPicker(...)`
func (m *Model) openRowPicker(title, verb string, targets []domain.Scope, choose func(domain.Scope) tea.Cmd) tea.Cmd {
	m.rowPicker = rowpicker.New(title, verb, targets)
	m.rowPickerPending = rowPickerPending{choose: choose, returnView: m.viewMode}
	m.viewMode = common.RowPickerView
	return nil
}

// dismissRowPicker closes the picker, restoring the prior view.
func (m *Model) dismissRowPicker() {
	m.viewMode = m.rowPickerPending.returnView
	m.rowPicker = rowpicker.Model{}
	m.rowPickerPending = rowPickerPending{}
}

// updateRowPickerView resolves a message while the picker is open: the
// picker decides (rowpicker.Model.Update), and a choice closes it and
// runs the pending choose while esc just closes it. ctrl+c quits.
func (m *Model) updateRowPickerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok && key.Matches(km, keys.Quit) {
		return m, m.quit()
	}
	var res rowpicker.Result
	m.rowPicker, res = m.rowPicker.Update(msg, m.modalMaxBody())
	switch res.Outcome {
	case rowpicker.Chosen:
		choose := m.rowPickerPending.choose
		m.dismissRowPicker()
		return m, choose(res.Scope)
	case rowpicker.Cancelled:
		m.dismissRowPicker()
	case rowpicker.None:
	}
	return m, nil
}

// pickerRows is how many target rows the picker shows before scrolling.
func (m *Model) pickerRows() int { return m.rowPicker.Rows(m.modalMaxBody()) }

// scrollWindow returns the first visible row of a window of n rows that
// keeps cursor in view (view.ScrollWindow).
func scrollWindow(cursor, offset, n int) int { return view.ScrollWindow(cursor, offset, n) }

// rowPickerView renders the picker in the theme.
func (m *Model) rowPickerView() string {
	p := m.theme.Palette
	return m.rowPicker.View(m.modalMaxBody(), rowpicker.Styles{Muted: p.Muted, Accent: p.Accent})
}
