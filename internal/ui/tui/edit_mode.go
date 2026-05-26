package tui

import (
	tea "github.com/charmbracelet/bubbletea"

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

// enterAliasMode enters the alias-completion (command) mode: starts
// from an empty input and offers the category alias list as
// completions.
func (m *Model) enterAliasMode() tea.Cmd {
	m.beginEditInput(common.AliasTarget)
	m.textInput.Reset()
	m.textInput.ShowSuggestions = len(domain.Aliases) > 0
	m.textInput.SetSuggestions(domain.Aliases)
	return nil
}

// enterFilterMode enters the filter mode: keeps the current input
// so re-entering filter mode preserves the partial. When the input
// is non-empty, also clears any active filter/scope
// (backToLastState) so the new typing matches against the full
// unscoped table.
func (m *Model) enterFilterMode() tea.Cmd {
	m.beginEditInput(common.FilterTarget)
	keys := domain.Aliases
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
		return m.updateRowsAsync()
	}
	return nil
}

func (m *Model) exitEditMode(resetInput bool) {
	if m.editTarget == common.AliasTarget || resetInput {
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
