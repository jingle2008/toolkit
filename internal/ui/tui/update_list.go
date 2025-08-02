// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
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
	case deleteErrMsg:
		m.handleDeleteErrMsg(msg)
	case deleteDoneMsg:
		m.handleDeleteDoneMsg(msg)
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

// handleNormalKeys processes key events in Normal mode.
func (m *Model) handleNormalKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Dispatch table for key handlers
	keyHandlers := []struct {
		match  key.Binding
		action func() []tea.Cmd
	}{
		{keys.Quit, func() []tea.Cmd { return []tea.Cmd{tea.Quit} }},
		{keys.BackHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(-1)} }},
		{keys.FwdHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(1)} }},
		{keys.NextCategory, func() []tea.Cmd { return []tea.Cmd{m.handleNextCategory()} }},
		{keys.PrevCategory, func() []tea.Cmd { return []tea.Cmd{m.handlePrevCategory()} }},
		{keys.FilterMode, func() []tea.Cmd { m.enterEditMode(common.FilterTarget); return nil }},
		{keys.PasteFilter, func() []tea.Cmd { return []tea.Cmd{m.pasteFilter()} }},
		{keys.CommandMode, func() []tea.Cmd { m.enterEditMode(common.AliasTarget); return nil }},
		{keys.ViewDetails, func() []tea.Cmd { m.enterDetailView(); return nil }},
		{keys.Confirm, func() []tea.Cmd { return []tea.Cmd{m.enterContext()} }},
		{keys.Help, func() []tea.Cmd { m.enterHelpView(); return nil }},
		{keys.SortName, func() []tea.Cmd { return []tea.Cmd{m.sortTableByColumn(common.NameCol)} }},
		{keys.ToggleAlias, func() []tea.Cmd { return m.toggleAliases() }},
		{keys.ExportCSV, func() []tea.Cmd { return m.enterExportView() }},
		{keys.Delete, func() []tea.Cmd { return m.handleDelete() }},
	}

	for _, h := range keyHandlers {
		if key.Matches(msg, h.match) {
			cmds = append(cmds, h.action()...)
			return cmds
		}
	}

	if key.Matches(msg, keys.CopyName) {
		actions.CopyItemName(m.getSelectedItem(), m.environment, m.logger)
	}

	cmds = append(cmds, m.handleAdditionalKeys(msg))
	return cmds
}

/*
handleDelete handles the generic delete action based on the current category.
For DedicatedAICluster, it deletes via SDK and removes the row locally.
*/
func (m *Model) handleDelete() []tea.Cmd {
	if m.category != domain.DedicatedAICluster {
		return nil
	}

	itemKey := getItemKey(m.category, m.table.SelectedRow())
	return m.DeleteDedicatedAICluster(itemKey)
}

func (m *Model) DeleteDedicatedAICluster(itemKey models.ItemKey) []tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	dac := item.(*models.DedicatedAICluster)
	prevState := dac.Status
	dac.Status = "Deleting"
	m.updateRows(false)
	return []tea.Cmd{
		func() tea.Msg {
			if err := actions.DeleteDedicatedAICluster(m.ctx, dac, m.environment, m.logger); err != nil {
				return deleteErrMsg{
					err:       err,
					key:       itemKey,
					prevState: prevState,
				}
			}
			return deleteDoneMsg{key: itemKey}
		},
	}
}

func (m *Model) handleDeleteErrMsg(msg deleteErrMsg) {
	m.logger.Errorw("failed to delete DAC", "key", msg.key, "error", msg.err)
	item := findItem(m.dataset, m.category, msg.key)
	dac := item.(*models.DedicatedAICluster)
	dac.Status = msg.prevState
	m.updateRows(false)
}

func (m *Model) handleDeleteDoneMsg(msg deleteDoneMsg) {
	deleteItem(m.dataset, m.category, msg.key)
	idx := m.table.Cursor()
	if idx+1 >= len(m.table.Rows()) {
		m.table.MoveUp(1)
	}
	m.updateRows(false)
}

func (m *Model) toggleAliases() []tea.Cmd {
	if m.category == domain.Alias {
		return []tea.Cmd{m.moveHistory(-1)}
	} else {
		return m.updateCategory(domain.Alias)
	}
}

func (m *Model) enterExportView() []tea.Cmd {
	m.lastViewMode = m.viewMode
	m.viewMode = common.ExportView

	var cmd tea.Cmd
	if m.dirPicker.Path != "" {
		cmd = func() tea.Msg {
			return tea.KeyMsg{Type: tea.KeyType(tea.KeyBackspace)}
		}
	} else {
		cmd = m.dirPicker.Init()
	}
	return []tea.Cmd{cmd}
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
	return tea.Sequence(m.updateCategory(category)...)
}

func (m *Model) handlePrevCategory() tea.Cmd {
	prev := int(m.category) - 1
	if prev < int(domain.Tenant) {
		prev = int(domain.DedicatedAICluster)
	}
	category := domain.Category(prev)
	return tea.Sequence(m.updateCategory(category)...)
}

// handleEditKeys processes key events in Edit mode.
func (m *Model) handleEditKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	updatedTextInput, cmd := m.textInput.Update(msg)
	m.textInput = &updatedTextInput
	cmds = append(cmds, cmd)

	switch {
	case msg.Type == tea.KeyCtrlC:
		cmds = append(cmds, tea.Quit)
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

// maxHistory is the maximum number of entries to keep in navigation history.
const maxHistory = 20

// pushHistory records a category change, discarding any
// "future" items if we are not at the end of the list.
// It also enforces a cap of maxHistory entries.
func (m *Model) pushHistory(cat domain.Category) {
	// ignore dups
	if m.historyIdx >= 0 && m.history[m.historyIdx] == cat {
		return
	}
	// truncate forward part when user branches
	if m.historyIdx+1 < len(m.history) {
		m.history = m.history[:m.historyIdx+1]
	}
	m.history = append(m.history, cat)
	m.historyIdx = len(m.history) - 1

	// Cap history size
	if len(m.history) > maxHistory {
		shift := len(m.history) - maxHistory
		m.history = m.history[shift:]
		m.historyIdx -= shift
		if m.historyIdx < 0 {
			m.historyIdx = 0
		}
	}
}

// moveHistory moves idx ±1 (dir = -1 back, +1 forward)
// and returns a tea.Cmd that switches category WITHOUT recording.
func (m *Model) moveHistory(dir int) tea.Cmd {
	target := m.historyIdx + dir
	if target < 0 || target >= len(m.history) {
		return nil // out of bounds
	}
	m.historyIdx = target
	cat := m.history[m.historyIdx]
	return tea.Sequence(m.updateCategoryNoHist(cat)...)
}
