// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmds = append(cmds, m.handleKeyMsg(msg)...)
	case DataMsg:
		m.handleDataMsg(msg)
	case FilterMsg:
		m.handleFilterMsg(msg)
	case SetFilterMsg:
		cmds = append(cmds, m.handleSetFilterMsg(msg))
	case ErrMsg:
		m.handleErrMsg(msg)
	case spinner.TickMsg:
		cmds = append(cmds, m.handleSpinnerTickMsg(msg))
	default:
		// Future-proof: ignore unknown message types
	}

	updatedTable, cmd := m.table.Update(msg)
	m.table = &updatedTable
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	if m.pendingTasks > 0 {
		return cmds
	}
	if key.Matches(msg, keys.Back) {
		m.backToLastState()
	}
	if m.inputMode == common.NormalInput {
		cmds = append(cmds, m.handleNormalKeys(msg)...)
	} else {
		cmds = append(cmds, m.handleEditKeys(msg)...)
	}
	return cmds
}

func (m *Model) handleDataMsg(msg DataMsg) {
	m.processData(msg)
}

func (m *Model) handleFilterMsg(msg FilterMsg) {
	if string(msg) == m.newFilter {
		FilterTable(m, string(msg))
	}
}

func (m *Model) handleSetFilterMsg(msg SetFilterMsg) tea.Cmd {
	m.newFilter = string(msg)
	m.textInput.SetValue(string(msg))
	return func() tea.Msg {
		return FilterMsg(msg)
	}
}

func (m *Model) handleErrMsg(msg ErrMsg) {
	m.pendingTasks--
	m.err = msg
}

func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) tea.Cmd {
	loadingSpinner, cmd := m.loadingSpinner.Update(msg)
	m.loadingSpinner = &loadingSpinner
	return cmd
}

// handleNormalKeys processes key events in Normal mode.
func (m *Model) handleNormalKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Dispatch table for key handlers
	keyHandlers := []struct {
		match  key.Binding
		action func() []tea.Cmd
	}{
		{keys.Quit, func() []tea.Cmd { return []tea.Cmd{tea.Quit} }},
		{keys.NextCategory, func() []tea.Cmd { return []tea.Cmd{m.handleNextCategory()} }},
		{keys.PrevCategory, func() []tea.Cmd { return []tea.Cmd{m.handlePrevCategory()} }},
		{keys.FilterList, func() []tea.Cmd { m.enterEditMode(common.FilterTarget); return nil }},
		{keys.PasteFilter, func() []tea.Cmd { return []tea.Cmd{m.pasteFilter()} }},
		{keys.JumpTo, func() []tea.Cmd { m.enterEditMode(common.AliasTarget); return nil }},
		{keys.ViewDetails, func() []tea.Cmd { m.enterDetailView(); return nil }},
		{keys.Confirm, func() []tea.Cmd { return []tea.Cmd{m.enterContext()} }},
		{keys.Help, func() []tea.Cmd { m.enterHelpView(); return nil }},
		{keys.CopyName, func() []tea.Cmd { m.copyItemName(m.getSelectedItem()); return nil }},
	}

	for _, h := range keyHandlers {
		if key.Matches(msg, h.match) {
			cmds = append(cmds, h.action()...)
			return cmds
		}
	}

	cmds = append(cmds, m.handleAdditionalKeys(msg))
	return cmds
}

func (*Model) pasteFilter() tea.Cmd {
	if clip, err := clipboard.ReadAll(); err == nil {
		if clip = strings.TrimSpace(clip); clip != "" {
			return setFilter(clip)
		}
	}
	return nil
}

func (m *Model) enterHelpView() {
	m.lastViewMode = m.viewMode
	m.viewMode = common.HelpView
}

func (m *Model) handleNextCategory() tea.Cmd {
	next := int(m.category) + 1
	if next > int(domain.DedicatedAICluster) {
		next = int(domain.Tenant)
	}
	category := domain.Category(next)
	return m.updateCategory(category)
}

func (m *Model) handlePrevCategory() tea.Cmd {
	prev := int(m.category) - 1
	if prev < int(domain.Tenant) {
		prev = int(domain.DedicatedAICluster)
	}
	category := domain.Category(prev)
	return m.updateCategory(category)
}

// handleEditKeys processes key events in Edit mode.
func (m *Model) handleEditKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	updatedTextInput, cmd := m.textInput.Update(msg)
	m.textInput = &updatedTextInput
	cmds = append(cmds, cmd)

	switch {
	case key.Matches(msg, keys.Confirm):
		if m.editTarget == common.AliasTarget {
			cmd := m.changeCategory()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.exitEditMode(m.editTarget == common.AliasTarget)
	case key.Matches(msg, keys.Back):
		m.exitEditMode(true)
	default:
		if m.editTarget == common.FilterTarget {
			cmds = append(cmds, DebounceFilter(m))
		}
	}
	return cmds
}
