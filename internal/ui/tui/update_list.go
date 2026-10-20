// Package toolkit: update_list.go
// Contains updateListView and related list view logic split from model_update.go.

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func (m *Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Keys the prefix machine takes (s and whatever follows it) must not
	// also reach the table, which would act on them on its own.
	if km, ok := msg.(tea.KeyPressMsg); ok && m.usesListKey(km.String()) {
		return m, tea.Batch(m.routeListMsg(msg)...)
	}
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
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case filterMsg:
		return []tea.Cmd{m.handleFilterMsg(msg)}
	case setFilterMsg:
		return []tea.Cmd{m.handleSetFilterMsg(msg)}
	case filterApplyMsg:
		return []tea.Cmd{m.handleFilterApplyMsg(msg)}
	case updateDoneMsg:
		m.handleUpdateDoneMsg(msg)
		return nil
	}
	return nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	// Prefix sequences come first, so esc with a prefix pending cancels
	// only the prefix, not the filter or scope.
	if m.inputMode == common.NormalInput {
		if seq, used := m.feedPrefix(msg.String()); used {
			return []tea.Cmd{m.handleListSequence(seq)}
		}
	}
	// While typing, esc closes the input and clears an applied filter,
	// but never leaves the scoped list: that takes a second esc.
	if key.Matches(msg, keys.Back) && (m.inputMode == common.NormalInput || m.filter != "") {
		cmd := m.goBack()
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
	m.gens.nextFilter()
	return func() tea.Msg {
		return filterMsg(normalized)
	}
}

func (m *Model) handleFilterApplyMsg(msg filterApplyMsg) tea.Cmd {
	// Only apply if this tick corresponds to the most recent debounce
	if msg.Gen == m.gens.filter {
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
		{keys.Quit, func() []tea.Cmd { return []tea.Cmd{m.quit()} }},
		{keys.BackHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(-1)} }},
		{keys.FwdHist, func() []tea.Cmd { return []tea.Cmd{m.moveHistory(1)} }},
		{keys.NextCategory, func() []tea.Cmd { return []tea.Cmd{m.handleNextCategory()} }},
		{keys.PrevCategory, func() []tea.Cmd { return []tea.Cmd{m.handlePrevCategory()} }},
		{keys.GoTo, func() []tea.Cmd { return []tea.Cmd{m.openGoTo()} }},
		{keys.FilterMode, func() []tea.Cmd { return []tea.Cmd{m.enterFilterMode()} }},
		{keys.PasteFilter, func() []tea.Cmd { return []tea.Cmd{m.pasteFilter()} }},
		{keys.CommandMode, func() []tea.Cmd { m.openPalette(); return nil }},
		{keys.ViewDetails, func() []tea.Cmd { return []tea.Cmd{m.enterDetailView()} }},
		{keys.Confirm, func() []tea.Cmd { return []tea.Cmd{m.enterContext()} }},
		{keys.Help, func() []tea.Cmd { m.enterHelpView(); return nil }},
		{keys.SortName, func() []tea.Cmd { return []tea.Cmd{m.sortTableByColumn(common.NameCol)} }},
		{keys.ToggleAlias, func() []tea.Cmd { return m.toggleAliases() }},
		{keys.TogglePreview, func() []tea.Cmd { return []tea.Cmd{m.togglePreview()} }},
		{keys.ExportCSV, func() []tea.Cmd { return m.enterExportView() }},
	}

	for _, h := range keyHandlers {
		if key.Matches(msg, h.match) {
			cmds = append(cmds, h.action()...)
			return cmds
		}
	}

	if key.Matches(msg, keys.QuickSwitch) {
		cmds = append(cmds, m.quickSwitch(msg))
		return cmds
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

func (*Model) pasteFilter() tea.Cmd {
	return func() tea.Msg {
		clip, err := clipboardReadAll()
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
	m.helpScroll = 0
	m.helpFilter, m.helpTyping = "", false
}

func (m *Model) handleNextCategory() tea.Cmd {
	return tea.Sequence(m.updateCategory(nextCategory(m.category))...)
}

func (m *Model) handlePrevCategory() tea.Cmd {
	return tea.Sequence(m.updateCategory(prevCategory(m.category))...)
}

// nextCategory is the Tab target from c: the following data category,
// wrapping from domain.LastDataCategory back to Tenant. Alias sits outside
// the cycle; Tab from it re-enters at Tenant.
func nextCategory(c domain.Category) domain.Category {
	if c < domain.Tenant || c >= domain.LastDataCategory {
		return domain.Tenant
	}
	return c + 1
}

// prevCategory is the Shift-Tab target from c, the inverse of nextCategory.
func prevCategory(c domain.Category) domain.Category {
	if c <= domain.Tenant || c > domain.LastDataCategory {
		return domain.LastDataCategory
	}
	return c - 1
}

// handleEditKeys processes key events in Edit mode.
func (m *Model) handleEditKeys(msg tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd
	updatedTextInput, cmd := m.textInput.Update(msg)
	m.textInput = &updatedTextInput
	cmds = append(cmds, cmd)

	switch {
	case msg.String() == "ctrl+c":
		cmds = append(cmds, m.quit())
	case key.Matches(msg, keys.Confirm):
		m.exitEditMode(false)
	case key.Matches(msg, keys.Back):
		m.exitEditMode(true)
	default:
		if m.editTarget == common.FilterTarget {
			cmds = append(cmds, DebounceFilter(m))
		}
	}
	return cmds
}
