package tui

import (
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func (m *Model) enterEditMode(target common.EditTarget) {
	m.table.Blur()
	m.inputMode = common.EditInput
	m.editTarget = target
	m.textInput.Focus()

	// Provide category suggestions using domain.Aliases.
	keys := domain.Aliases
	if target == common.AliasTarget {
		m.textInput.Reset()
	} else if len(m.textInput.Value()) > 0 {
		keys = append(keys, m.textInput.Value())
		m.backToLastState()
	}

	m.textInput.ShowSuggestions = len(keys) > 0
	m.textInput.SetSuggestions(keys)
}

func (m *Model) backToLastState() {
	if m.curFilter != "" {
		m.textInput.Reset()
		FilterTable(m, "")
	} else if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		m.context = nil
		m.updateRows(true)
	}
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
