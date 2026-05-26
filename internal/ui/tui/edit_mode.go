package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func (m *Model) enterEditMode(target common.EditTarget) tea.Cmd {
	m.table.Blur()
	m.inputMode = common.EditInput
	m.editTarget = target
	m.textInput.Focus()

	if target == common.AliasTarget {
		// Command mode: start from an empty input and offer the
		// category alias list as completions.
		m.textInput.Reset()
		m.textInput.ShowSuggestions = len(domain.Aliases) > 0
		m.textInput.SetSuggestions(domain.Aliases)
		return nil
	}

	// FilterTarget: keep the current input so re-entering filter mode
	// preserves the partial. When the input is non-empty, also clear
	// any active filter/scope (backToLastState) so the new typing
	// matches against the full unscoped table.
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
	if m.curFilter != "" {
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
