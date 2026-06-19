// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := m.routeListMsg(msg)

	updatedTable, cmd := m.table.Update(msg)
	m.table = &updatedTable
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) routeListMsg(msg tea.Msg) []tea.Cmd {
	// dataMsg, datasetLoadedMsg, and the typed *LoadedMsg family are
	// intercepted at the top of Update so they fire from any view —
	// they don't reach this router.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case filterMsg:
		return []tea.Cmd{m.handleFilterMsg(msg)}
	case setFilterMsg:
		return []tea.Cmd{m.handleSetFilterMsg(msg)}
	case filterApplyMsg:
		return []tea.Cmd{m.handleFilterApplyMsg(msg)}
	case deleteErrMsg:
		m.handleDeleteErrMsg(msg)
		return nil
	case deleteDoneMsg:
		m.handleDeleteDoneMsg(msg)
		return nil
	case updateDoneMsg:
		m.handleUpdateDoneMsg(msg)
		return nil
	default:
		return m.routeListAsyncMsg(msg)
	}
}

func (m *Model) routeListAsyncMsg(msg tea.Msg) []tea.Cmd {
	switch msg := msg.(type) {
	case gpuPoolScaleStartedMsg:
		m.handleGPUPoolScaleStartedMsg(msg)
	case gpuPoolScaleResultMsg:
		m.handleGPUPoolScaleResultMsg(msg)
	case cordonNodeResultMsg:
		m.handleCordonNodeResultMsg(msg)
	case drainNodeResultMsg:
		m.handleDrainNodeResultMsg(msg)
	case rebootNodeResultMsg:
		m.handleRebootNodeResultMsg(msg)
	}
	return nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	if key.Matches(msg, keys.Back) {
		cmd := m.backToLastState()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.inputMode == common.NormalInput {
		cmds = append(cmds, m.handleNormalKeys(msg)...)
	} else {
		cmds = append(cmds, m.handleEditKeys(msg)...)
	}
	return cmds
}

func (m *Model) handleFilterMsg(msg filterMsg) tea.Cmd {
	return filterTableAsync(m, string(msg))
}

func (m *Model) handleSetFilterMsg(msg setFilterMsg) tea.Cmd {
	val := string(msg)
	m.textInput.SetValue(val)
	normalized := strings.ToLower(val)
	// Invalidate any pending debounce tick that may be in-flight.
	m.filterGen++
	return func() tea.Msg {
		return filterMsg(normalized)
	}
}

func (m *Model) handleFilterApplyMsg(msg filterApplyMsg) tea.Cmd {
	// Only apply if this tick corresponds to the most recent debounce
	if msg.Gen == m.filterGen {
		return filterTableAsync(m, msg.Value)
	}
	return nil
}

// handleNormalKeys processes key events in Normal mode.
func (m *Model) handleNormalKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Dispatch table for key handlers
	keyHandlers := []struct {
		match  key.Binding
		action func() []tea.Cmd
	}{
		{keys.Quit, func() []tea.Cmd { return []tea.Cmd{func() tea.Msg { m.cancelInFlight(); return tea.QuitMsg{} }} }},
		{keys.BackHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(-1)} }},
		{keys.FwdHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(1)} }},
		{keys.NextCategory, func() []tea.Cmd { return []tea.Cmd{m.handleNextCategory()} }},
		{keys.PrevCategory, func() []tea.Cmd { return []tea.Cmd{m.handlePrevCategory()} }},
		{keys.Parent, func() []tea.Cmd { return []tea.Cmd{m.jumpToParent()} }},
		{keys.FilterMode, func() []tea.Cmd { return []tea.Cmd{m.enterFilterMode()} }},
		{keys.PasteFilter, func() []tea.Cmd { return []tea.Cmd{m.pasteFilter()} }},
		{keys.CommandMode, func() []tea.Cmd { m.enterAliasMode(); return nil }},
		{keys.ViewDetails, func() []tea.Cmd { return []tea.Cmd{m.enterDetailView()} }},
		{keys.Confirm, func() []tea.Cmd { return []tea.Cmd{m.enterContext()} }},
		{keys.Help, func() []tea.Cmd { m.enterHelpView(); return nil }},
		{keys.SortName, func() []tea.Cmd { return []tea.Cmd{m.sortTableByColumn(common.NameCol)} }},
		{keys.ToggleAlias, func() []tea.Cmd { return m.toggleAliases() }},
		{keys.ExportCSV, func() []tea.Cmd { return m.enterExportView() }},
	}

	for _, h := range keyHandlers {
		if key.Matches(msg, h.match) {
			cmds = append(cmds, h.action()...)
			return cmds
		}
	}

	if key.Matches(msg, keys.CopyName) {
		cmds = append(cmds, m.copyItemNameCmd(m.selectedItem()))
		return cmds
	}

	cmds = append(cmds, m.handleAdditionalKeys(msg))
	return cmds
}

func (m *Model) toggleAliases() []tea.Cmd {
	if m.category == domain.Alias {
		return []tea.Cmd{m.moveHistory(-1)}
	}
	return m.updateCategory(domain.Alias)
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
	return func() tea.Msg {
		clip, err := clipboard.ReadAll()
		if err != nil {
			return nil
		}
		clip = strings.TrimSpace(clip)
		if clip == "" {
			return nil
		}
		return setFilterMsg(clip)
	}
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
