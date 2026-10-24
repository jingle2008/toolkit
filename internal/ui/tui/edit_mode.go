package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// beginEditInput is the shared prelude for the two edit-mode entry
// points (alias and filter): switch the table into edit-input mode,
// stamp the target, and focus the text input.
func (m *Model) beginEditInput(target common.EditTarget) {
	m.table.Blur()
	m.inputMode = common.EditInput
	m.editTarget = target
	m.textInput.Focus()
}

// enterFilterMode enters the filter mode: keeps the current input
// so re-entering filter mode preserves the partial. When the input
// is non-empty, also clears any active filter/scope
// (backToLastState) so the new typing matches against the full
// unscoped table.
func (m *Model) enterFilterMode() tea.Cmd {
	m.beginEditInput(common.FilterTarget)
	keys := domain.AllAliases()
	var cmd tea.Cmd
	if len(m.textInput.Value()) > 0 {
		keys = append(keys, m.textInput.Value())
		cmd = m.backToLastState()
	}
	m.textInput.ShowSuggestions = len(keys) > 0
	m.textInput.SetSuggestions(keys)
	return cmd
}

func (m *Model) backToLastState() tea.Cmd {
	if m.filter != "" {
		m.textInput.Reset()
		return filterTableAsync(m, "")
	} else if m.scope != nil && m.scope.Category.IsScopeOf(m.category) {
		m.scope = nil
		m.syncHistoryScope()
		return m.updateRowsAsync()
	}
	return nil
}

// goBack is esc in the list: it clears the filter, or, in a scoped list,
// returns to the parent list with the cursor on the row drilled in from.
// Back in history when the parent is there, so ] still leads into the
// child: the parent shows under the scope it was visited with and the
// view it was left with, and landOn puts the cursor on the drilled-from
// row. The child left moves to just after the parent, so ] returns to it
// even when other children were visited in between. Otherwise the parent
// is pushed, keeping the drill scope, which lands the cursor there itself.
func (m *Model) goBack() tea.Cmd {
	if m.filter != "" || m.scope == nil || !m.scope.Category.IsScopeOf(m.category) {
		return m.backToLastState()
	}
	from := copyScope(m.scope)
	parent := from.Category
	for i := m.historyIdx - 1; i >= 0; i-- {
		if m.history[i].category == parent {
			m.saveView()
			child := m.history[m.historyIdx]
			m.history = slices.Concat(m.history[:i+1], []historyEntry{child}, m.history[m.historyIdx+1:])
			m.historyIdx = i
			m.scope = copyScope(m.history[i].scope)
			cmds := m.updateCategoryNoHist(parent)
			m.restoreView(m.history[i].view)
			m.landOn, m.landOnKey = from, nil // the drilled-from row, not the saved cursor
			return tea.Sequence(cmds...)
		}
	}
	return tea.Sequence(m.updateCategory(parent)...)
}

func (m *Model) exitEditMode(resetInput bool) {
	if resetInput {
		m.textInput.SetSuggestions([]string{})
		m.textInput.ShowSuggestions = false
	}

	m.inputMode = common.NormalInput
	m.editTarget = common.NoneTarget
	if resetInput {
		m.textInput.Reset()
	}
	m.textInput.Blur()
	m.table.Focus()
}
