// Package toolkit implements the update/reduce logic for the Model.
package toolkit

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/app/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

/*
loadRequest is a command for loading category data using the model's context.
*/
type loadRequest struct {
	category domain.Category
	model    *Model
}

func (r loadRequest) Run() tea.Msg {
	var (
		data interface{}
		err  error
	)
	switch r.category {
	case domain.BaseModel:
		data, err = r.model.loader.LoadBaseModels(r.model.contextCtx, r.model.repoPath, r.model.environment)
	case domain.GpuPool:
		data, err = r.model.loader.LoadGpuPools(r.model.contextCtx, r.model.repoPath, r.model.environment)
	case domain.GpuNode:
		data, err = r.model.loader.LoadGpuNodes(r.model.contextCtx, r.model.kubeConfig, r.model.environment)
	case domain.DedicatedAICluster:
		data, err = r.model.loader.LoadDedicatedAIClusters(r.model.contextCtx, r.model.kubeConfig, r.model.environment)
	}
	if err != nil {
		return errMsg{err}
	}
	return dataMsg{data}
}

/*
Update implements the tea.Model interface and updates the Model state in response to a message.
*/
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.reduce(msg)
}

// reduce is a pure state reducer for Model, used for testability.
func (m *Model) reduce(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.reLayout = true
		m.updateLayout(msg.Width, msg.Height)
	}
	if !m.chosen {
		return updateListView(msg, m)
	}
	return updateDetailView(msg, m)
}

func (m *Model) enterEditMode(target EditTarget) {
	m.table.Blur()
	m.mode = Edit
	m.target = target
	m.textInput.Focus()

	keys := make([]string, 0, len(categoryMap))
	if target == Alias {
		m.textInput.Reset()
		for k := range categoryMap {
			keys = append(keys, k)
		}
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
		m.filterTable("")
	} else if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		m.context = nil
		m.updateRows()
	}
}

func (m *Model) exitEditMode(resetInput bool) {
	if m.target == Alias || resetInput {
		m.textInput.SetSuggestions([]string{})
		m.textInput.ShowSuggestions = false
	}

	m.mode = Normal
	m.target = None
	if resetInput {
		m.textInput.Reset()
	}
	m.textInput.Blur()
	m.table.Focus()
}

func (m *Model) filterTable(filter string) {
	if filter == m.curFilter {
		return
	}

	m.curFilter = filter
	m.updateRows()
}

func (m *Model) changeCategory() tea.Cmd {
	text := strings.ToLower(m.textInput.Value())

	category, ok := categoryMap[text]
	if !ok {
		return nil
	}

	return m.updateCategory(category)
}

func (m *Model) debounceFilter() tea.Cmd {
	m.newFilter = strings.ToLower(m.textInput.Value())

	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return filterMsg{m.newFilter}
	})
}

func (m *Model) updateCategory(category domain.Category) tea.Cmd {
	if m.category == category {
		return nil
	}

	m.category = category
	m.keys.Category = category

	switch m.category {
	case domain.BaseModel:
		if m.dataset.BaseModelMap == nil {
			return loadRequest{category: domain.BaseModel, model: m}.Run
		}
	case domain.GpuPool:
		if m.dataset.GpuPools == nil {
			return loadRequest{category: domain.GpuPool, model: m}.Run
		}
	case domain.GpuNode:
		if m.dataset.GpuNodeMap == nil {
			return loadRequest{category: domain.GpuNode, model: m}.Run
		}
	case domain.DedicatedAICluster:
		if m.dataset.DedicatedAIClusterMap == nil {
			return loadRequest{category: domain.DedicatedAICluster, model: m}.Run
		}
	}

	// trigger refresh of the table
	return func() tea.Msg {
		return dataMsg{}
	}
}

func (m *Model) enterContext() tea.Cmd {
	target := m.table.SelectedRow()[0]
	appContext := domain.AppContext{
		Category: m.category,
		Name:     target,
	}
	switch {
	case m.category.IsScope():
		m.context = &appContext
		return m.updateCategory(m.category.ScopedCategories()[0])
	case m.category == domain.Environment:
		env := *utils.FindByName(m.dataset.Environments, target)
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

func (m *Model) exitDetailView() {
	m.chosen = false

	if m.reLayout {
		m.reLayout = false
		m.updateLayout(m.viewWidth, m.viewHeight)
	}
}

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

func (m *Model) updateColumns() {
	m.headers = getHeaders(m.category)
	columns := make([]table.Column, len(m.headers))

	remaining := m.table.Width()
	for i, header := range m.headers {
		width := remaining
		// last item will take the remaining space
		if i+1 < len(m.headers) {
			width = int(math.Floor(float64(m.table.Width()) * float64(header.ratio)))
			remaining -= width
		}

		width -= m.styles.Header.GetHorizontalFrameSize()
		columns[i] = table.Column{Title: header.text, Width: width}
	}

	table.WithColumns(columns)(m.table)
}

func (m *Model) updateRows() {
	rows := getTableRows(m.loggerCtx(), m.dataset, m.category, m.context, m.curFilter)
	table.WithRows(rows)(m.table)

	m.table.GotoTop()
}

func (m *Model) refreshDisplay() {
	m.curFilter = ""    // reset current filter
	m.newFilter = ""    // ignore future filter message
	m.textInput.Reset() // reset filter display

	m.updateColumns()
	m.updateRows()
}

func (m *Model) processData(msg dataMsg) {
	//nolint:exhaustive
	switch data := msg.data.(type) {
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

	m.refreshDisplay()
}

func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) {
	//nolint:exhaustive
	if m.category == domain.BaseModel {
		if key.Matches(msg, m.keys.ViewModelArtifacts) {
			item := m.getCurrentItem()
			if bm, ok := item.(*models.BaseModel); ok {
				m.loggerCtx().Info("view_model_artifacts",
					zap.String("model", bm.Name),
					zap.String("version", bm.Version),
					zap.String("type", bm.Type),
				)
			} else {
				m.loggerCtx().Info("view_model_artifacts", zap.Any("item", item))
			}
		}
	}
}

func (m *Model) getCurrentItem() interface{} {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}
