// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
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
		cmds = append(cmds, m.handleSpinnerTickMsg(msg)...)
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
	if m.loading {
		return cmds
	}
	if msg.String() == "esc" {
		m.backToLastState()
	}
	if m.mode == Normal {
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
	if msg.Text == m.newFilter {
		FilterTable(m, msg.Text)
	}
}

func (m *Model) handleSetFilterMsg(msg SetFilterMsg) tea.Cmd {
	m.newFilter = msg.Text
	m.textInput.SetValue(msg.Text)
	return func() tea.Msg {
		return FilterMsg(msg)
	}
}

func (m *Model) handleErrMsg(msg ErrMsg) {
	m.err = msg.Err
}

func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) []tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.loadingSpinner, cmd = m.loadingSpinner.Update(msg)
	cmds = append(cmds, cmd)
	return cmds
}

// handleNormalKeys processes key events in Normal mode.
func (m *Model) handleNormalKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	switch {
	case key.Matches(msg, m.keys.Quit):
		return []tea.Cmd{tea.Quit}
	case key.Matches(msg, m.keys.NextCategory):
		cmds = append(cmds, m.handleNextCategory())
	case key.Matches(msg, m.keys.PrevCategory):
		cmds = append(cmds, m.handlePrevCategory())
	case key.Matches(msg, m.keys.FilterItems):
		m.enterEditMode(Filter)
	case key.Matches(msg, m.keys.JumpTo):
		m.enterEditMode(Alias)
	case key.Matches(msg, m.keys.ViewDetails):
		m.enterDetailView()
	case key.Matches(msg, m.keys.ApplyContext):
		cmd := m.enterContext()
		cmds = append(cmds, cmd)
	default:
		m.handleAdditionalKeys(msg)
	}
	return cmds
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

	switch msg.String() {
	case "enter":
		if m.target == Alias {
			cmd := m.changeCategory()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		m.exitEditMode(m.target == Alias)
	case "esc":
		m.exitEditMode(true)
	default:
		if m.target == Filter {
			cmds = append(cmds, DebounceFilter(m))
		}
	}
	return cmds
}
