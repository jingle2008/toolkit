// Package tui contains reducer and event logic for the Model.
// This file contains methods for state transitions, event handling, and UI updates.
package tui

import (
	"math"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// reusable refresh command emitting an empty DataMsg
var refreshCmd tea.Cmd = func() tea.Msg { return DataMsg{} }

// updateRows updates the table rows based on the current model state.
func (m *Model) updateRows() {
	rows := getTableRows(m.logger, m.dataset, m.category, m.context, m.curFilter)
	table.WithRows(rows)(m.table)
	m.table.GotoTop()
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

// updateLayout recalculates the layout for the view and table.
func (m *Model) updateLayout(w, h int) {
	m.viewWidth, m.viewHeight = w, h
	m.help.Width = w
	var borderStyle lipgloss.Border
	if m.chosen {
		borderStyle = m.viewport.Style.GetBorderStyle()
	} else {
		borderStyle = m.baseStyle.GetBorderStyle()
	}
	borderWidth := borderStyle.GetLeftSize() + borderStyle.GetRightSize()
	borderHeight := borderStyle.GetTopSize() + borderStyle.GetBottomSize()
	statusHeight := lipgloss.Height(m.statusView())
	helpHeight := lipgloss.Height(m.help.View(m.keys))
	top := statusHeight + helpHeight
	if m.chosen {
		m.viewport.Width = w - borderWidth
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
	}
	m.loading = false
	m.refreshDisplay()
}

// handleAdditionalKeys processes extra key events for the current category.
func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) {
	if m.category == domain.BaseModel {
		if key.Matches(msg, m.keys.ViewModelArtifacts) {
			item := m.getCurrentItem()
			if bm, ok := item.(*models.BaseModel); ok {
				m.logger.Infow("view_model_artifacts",
					"model", bm.Name,
					"version", bm.Version,
					"type", bm.Type,
				)
			} else {
				m.logger.Infow("view_model_artifacts", "item", item)
			}
		}
	}
}

// getCurrentItem returns the currently selected item in the table.
func (m *Model) getCurrentItem() interface{} {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}

// updateCategory changes the current category and loads data if needed.
func (m *Model) updateCategory(category domain.Category) tea.Cmd {
	m.category = category
	m.keys.Category = category
	switch m.category {
	case domain.BaseModel:
		return m.handleBaseModelCategory()
	case domain.GpuPool:
		return m.handleGpuPoolCategory()
	case domain.GpuNode:
		return m.handleGpuNodeCategory()
	case domain.DedicatedAICluster:
		return m.handleDedicatedAIClusterCategory()
	default:
		return refreshCmd
	}
}

func (m *Model) handleBaseModelCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.BaseModelMap == nil {
		m.loading = true
		return loadRequest{category: domain.BaseModel, model: m}.Run
	}
	return refreshCmd
}

func (m *Model) handleGpuPoolCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.GpuPools == nil {
		m.loading = true
		return loadRequest{category: domain.GpuPool, model: m}.Run
	}
	return refreshCmd
}

func (m *Model) handleGpuNodeCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.GpuNodeMap == nil {
		m.loading = true
		return loadRequest{category: domain.GpuNode, model: m}.Run
	}
	return refreshCmd
}

func (m *Model) handleDedicatedAIClusterCategory() tea.Cmd {
	if m.dataset == nil || m.dataset.DedicatedAIClusterMap == nil {
		m.loading = true
		return loadRequest{category: domain.DedicatedAICluster, model: m}.Run
	}
	return refreshCmd
}

// enterDetailView switches the model into detail view mode.
func (m *Model) enterDetailView() {
	m.chosen = true
	m.choice = getItemKey(m.category, m.table.SelectedRow())
	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	} else {
		m.updateContent(0)
	}
}

// exitDetailView exits detail view mode.
func (m *Model) exitDetailView() {
	m.chosen = false
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
	target := m.table.SelectedRow()[0]
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
			// reset env-bounded data
			m.dataset.BaseModelMap = nil
			m.dataset.GpuPools = nil
			m.dataset.GpuNodeMap = nil
			m.dataset.DedicatedAIClusterMap = nil
			return tea.Batch(
				m.updateCategory(domain.BaseModel),
			)
		}
	default:
		m.enterDetailView()
	}
	return nil
}
