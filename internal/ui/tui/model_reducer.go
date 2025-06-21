// Package tui contains reducer and event logic for the Model.
// This file contains methods for state transitions, event handling, and UI updates.
package tui

import (
	"fmt"
	"math"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

var refreshDataCmd tea.Cmd = func() tea.Msg { return DataMsg{} }

// updateRows updates the table rows based on the current model state.
func (m *Model) updateRows() {
	rows := getTableRows(m.logger, m.dataset, m.category, m.context, m.curFilter)
	table.WithRows(rows)(m.table)

	idx := m.findContextIndex(rows)
	if idx >= 0 {
		m.table.SetCursor(idx)
		m.table.MoveDown(0) // scroll to the context row
	} else {
		m.table.GotoTop()
	}
}

// updateColumns updates the table columns based on the current category.
func (m *Model) updateColumns() {
	m.headers = getHeaders(m.category)
	columns := make([]table.Column, len(m.headers))
	remaining := m.table.Width()
	for i, header := range m.headers {
		width := remaining
		if i+1 < len(m.headers) {
			width = int(math.Floor(float64(m.table.Width()) * float64(header.ratio)))
			remaining -= width
		}
		width -= m.styles.Header.GetHorizontalFrameSize()
		columns[i] = table.Column{Title: header.text, Width: width}
	}
	table.WithColumns(columns)(m.table)
}

// findContextIndex returns the index of the row to move the cursor to, based on environment or context.
func (m *Model) findContextIndex(rows []table.Row) int {
	if len(rows) == 0 {
		return -1
	}

	var name string
	switch {
	case m.category == domain.Environment:
		name = m.environment.GetName()
	case m.context != nil && m.category == m.context.Category:
		name = m.context.Name
	default:
		return -1
	}

	for i, r := range rows {
		if r[0] == name {
			return i
		}
	}

	return -1
}

// updateLayout recalculates the layout for the view and table.
func (m *Model) updateLayout(w, h int) {
	m.viewWidth, m.viewHeight = w, h
	m.help.Width = w
	var borderStyle lipgloss.Border
	if m.viewMode == common.DetailsView {
		borderStyle = m.viewport.Style.GetBorderStyle()
	} else {
		borderStyle = m.baseStyle.GetBorderStyle()
	}
	borderWidth := borderStyle.GetLeftSize() + borderStyle.GetRightSize()
	borderHeight := borderStyle.GetTopSize() + borderStyle.GetBottomSize()
	statusHeight := lipgloss.Height(m.statusView())
	helpHeight := lipgloss.Height(m.help.View(m.keys))
	top := statusHeight + helpHeight
	if m.viewMode == common.DetailsView {
		m.viewport.Width = w // - borderWidth, seems a bug in bubbletea
		m.viewport.Height = h - borderHeight - top
		m.updateContent(w - borderWidth)
	} else {
		headerHeight := lipgloss.Height(m.styles.Header.Render("test"))
		table.WithWidth(w - borderWidth)(m.table)
		table.WithHeight(h - borderHeight - headerHeight - top)(m.table)
		m.updateColumns()
		m.table.UpdateViewport()
	}
}

// refreshDisplay resets filters and updates columns and rows.
func (m *Model) refreshDisplay() {
	m.curFilter = ""
	m.newFilter = ""
	m.textInput.Reset()
	m.updateColumns()
	m.updateRows()
}

// processData updates the model's dataset based on the incoming DataMsg.
func (m *Model) processData(msg DataMsg) {
	switch data := msg.Data.(type) {
	case *models.Dataset:
		m.dataset = data
	case map[string]*models.BaseModel:
		m.dataset.BaseModelMap = data
	case []models.GpuPool:
		m.dataset.GpuPools = data
	case map[string][]models.GpuNode:
		m.dataset.GpuNodeMap = data
	case map[string][]models.DedicatedAICluster:
		m.dataset.SetDedicatedAIClusterMap(data)
	case models.TenancyOverrideGroup:
		m.dataset.Tenants = data.Tenants
		m.dataset.LimitTenancyOverrideMap = data.LimitTenancyOverrideMap
		m.dataset.ConsolePropertyTenancyOverrideMap = data.ConsolePropertyTenancyOverrideMap
		m.dataset.PropertyTenancyOverrideMap = data.PropertyTenancyOverrideMap
	}

	if msg.Data != nil {
		m.pendingTasks--
	}
	m.refreshDisplay()
}

// handleAdditionalKeys processes extra key events for the current category.
func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) {
	if !key.Matches(msg, m.keys.Context...) {
		return
	}

	item := m.getSelectedItem()
	switch {
	case key.Matches(msg, keys.CopyTenant):
		m.copyTenantID(item)
	case key.Matches(msg, keys.CopyValue):
		m.copyItemValue(item)
	}
}

func (m *Model) copyItemName(item any) {
	if item == nil {
		m.logger.Errorw("no item selected for copying name", "category", m.category)
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		id := fmt.Sprintf("ocid1.generativeaidedicatedaicluster.%s.%s.%s",
			m.environment.Realm, m.environment.Region, dac.Name)
		if err := clipboard.WriteAll(id); err != nil {
			m.logger.Errorw("failed to copy id to clipboard", "error", err)
		}
	} else if to, ok := item.(models.NamedItem); ok {
		if err := clipboard.WriteAll(to.GetName()); err != nil {
			m.logger.Errorw("failed to copy name to clipboard", "error", err)
		}
	} else {
		m.logger.Errorw("unsupported item type for copying name", "item", item)
	}
}

// copyTenantId copies the tenant ID from the current row to the clipboard if available.
func (m *Model) copyTenantID(item any) {
	if item == nil {
		m.logger.Errorw("no item selected for copying tenant ID", "category", m.category)
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		tenantID := fmt.Sprintf("ocid1.tenancy.%s..%s", m.environment.Realm, dac.TenantID)
		if err := clipboard.WriteAll(tenantID); err != nil {
			m.logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else if to, ok := item.(models.TenancyOverride); ok {
		if err := clipboard.WriteAll(to.GetTenantID()); err != nil {
			m.logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else {
		m.logger.Errorw("unsupported item type for copying tenant ID", "item", item)
	}
}

func (m *Model) copyItemValue(item any) {
	if item == nil {
		m.logger.Errorw("no item selected for copying value", "category", m.category)
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		value := dac.UnitShape
		if value == "" {
			value = dac.Profile
		}
		if err := clipboard.WriteAll(value); err != nil {
			m.logger.Errorw("failed to copy value to clipboard", "error", err)
		}
	} else if to, ok := item.(*models.PropertyTenancyOverride); ok {
		if err := clipboard.WriteAll(to.GetValue()); err != nil {
			m.logger.Errorw("failed to copy value to clipboard", "error", err)
		}
	} else {
		m.logger.Errorw("unsupported item type for copying value", "item", item)
	}
}

// getSelectedItem returns the currently selected item in the table.
func (m *Model) getSelectedItem() any {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}

// updateCategory changes the current category and loads data if needed.
func (m *Model) updateCategory(category domain.Category) tea.Cmd {
	m.category = category
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	switch m.category { //nolint:exhaustive
	case domain.BaseModel:
		return m.handleBaseModelCategory()
	case domain.GpuPool:
		return m.handleGpuPoolCategory()
	case domain.GpuNode:
		return m.handleGpuNodeCategory()
	case domain.DedicatedAICluster:
		return m.handleDedicatedAIClusterCategory()
	case domain.Tenant, domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		return m.handleTenancyOverridesGroup()
	case domain.LimitRegionalOverride:
		return m.handleLimitRegionalOverrideCategory()
	case domain.ConsolePropertyRegionalOverride:
		return m.handleConsolePropertyRegionalOverrideCategory()
	case domain.PropertyRegionalOverride:
		return m.handlePropertyRegionalOverrideCategory()
	default:
		return refreshDataCmd
	}
}

// Lazy loaders for realm-specific categories
func (m *Model) handleTenancyOverridesGroup() tea.Cmd {
	if m.dataset == nil ||
		m.dataset.Tenants == nil ||
		m.dataset.LimitTenancyOverrideMap == nil ||
		m.dataset.ConsolePropertyTenancyOverrideMap == nil ||
		m.dataset.PropertyTenancyOverrideMap == nil {
		m.pendingTasks++
		return loadRequest{category: domain.Tenant, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleLimitRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.LimitRegionalOverrides == nil {
		m.pendingTasks++
		return loadRequest{category: domain.LimitRegionalOverride, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleConsolePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.ConsolePropertyRegionalOverrides == nil {
		m.pendingTasks++
		return loadRequest{category: domain.ConsolePropertyRegionalOverride, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handlePropertyRegionalOverrideCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.PropertyRegionalOverrides == nil {
		m.pendingTasks++
		return loadRequest{category: domain.PropertyRegionalOverride, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleBaseModelCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModelMap == nil {
		m.pendingTasks++
		return loadRequest{category: domain.BaseModel, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleGpuPoolCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.GpuPools == nil {
		m.pendingTasks++
		return loadRequest{category: domain.GpuPool, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleGpuNodeCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.GpuNodeMap == nil {
		m.pendingTasks++
		return loadRequest{category: domain.GpuNode, model: m}.Run
	}
	return refreshDataCmd
}

func (m *Model) handleDedicatedAIClusterCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil {
		m.pendingTasks++
		return loadRequest{category: domain.DedicatedAICluster, model: m}.Run
	}
	return refreshDataCmd
}

// enterDetailView switches the model into detail view mode.
func (m *Model) enterDetailView() {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return
	}

	m.viewMode = common.DetailsView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	m.choice = getItemKey(m.category, row)
	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	} else {
		m.updateContent(0)
	}
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, m.viewMode)
	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	}
}

// changeCategory parses the text input and updates the category.
func (m *Model) changeCategory() tea.Cmd {
	text := m.textInput.Value()
	category, err := domain.ParseCategory(text)
	if err != nil {
		return nil
	}

	if m.category == category {
		return nil
	}
	return m.updateCategory(category)
}

// enterContext moves the model into a new context based on the selected row.
func (m *Model) enterContext() tea.Cmd {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}

	target := row[0]
	appContext := domain.ToolkitContext{
		Category: m.category,
		Name:     target,
	}
	switch {
	case m.category.IsScope():
		m.context = &appContext
		return m.updateCategory(m.category.ScopedCategories()[0])
	case m.category == domain.Environment:
		env := *collections.FindByName(m.dataset.Environments, target)
		if !m.environment.Equals(env) {
			m.environment = env
			m.dataset.ResetScopedData()
			return tea.Batch(
				m.updateCategory(domain.Tenant),
			)
		}
	default:
		m.enterDetailView()
	}
	return nil
}
