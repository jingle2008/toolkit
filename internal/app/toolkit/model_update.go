// Package toolkit implements the update/reduce logic for the Model.
package toolkit

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

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

func updateListView(msg tea.Msg, m *Model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.backToLastState()
		}

		if m.mode == Normal {
			switch {

			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit

			case key.Matches(msg, m.keys.NextCategory):
				category := (m.category + 1) % numCategories
				cmds = append(cmds, m.updateCategory(category))

			case key.Matches(msg, m.keys.PrevCategory):
				category := (m.category + numCategories - 1) % numCategories
				cmds = append(cmds, m.updateCategory(category))

			case key.Matches(msg, m.keys.FilterItems):
				m.enterEditMode(Filter)

			case key.Matches(msg, m.keys.JumpTo):
				m.enterEditMode(Alias)

			case key.Matches(msg, m.keys.ViewDetails):
				m.enterDetailView()

			case key.Matches(msg, m.keys.ApplyContext):
				cmd = m.enterContext()
				cmds = append(cmds, cmd)

			default:
				m.handleAdditionalKeys(msg)
			}
		} else {
			updatedTextInput, cmd := m.textInput.Update(msg)
			m.textInput = &updatedTextInput
			cmds = append(cmds, cmd)

			switch msg.String() {

			case "enter":
				if m.target == Alias {
					cmd = m.changeCategory()
					if cmd == nil {
						break
					}
					cmds = append(cmds, cmd)
				}
				m.exitEditMode(m.target == Alias)

			case "esc":
				m.exitEditMode(true)

			default:
				if m.target == Filter {
					cmds = append(cmds, m.debounceFilter())
				}
			}
		}

	case dataMsg:
		m.processData(msg)

	case filterMsg:
		if msg.text == m.newFilter {
			m.filterTable(msg.text)
		}

	case errMsg:
		m.err = msg.err
	}

	updatedTable, cmd := m.table.Update(msg)
	m.table = &updatedTable
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func updateDetailView(msg tea.Msg, m *Model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" {
			m.exitDetailView()
		}
	}

	updatedViewport, cmd := m.viewport.Update(msg)
	m.viewport = &updatedViewport
	return m, cmd
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

func (m *Model) updateCategory(category Category) tea.Cmd {
	if m.category == category {
		return nil
	}

	m.category = category
	m.keys.Category = category
	return m.ensureCategory
}

func (m *Model) ensureCategory() tea.Msg {
	var data interface{}
	var err error

	//nolint:exhaustive
	switch m.category {
	case BaseModel:
		if m.dataset.BaseModelMap == nil {
			data, err = utils.LoadBaseModels(context.Background(), m.repoPath, m.environment)
		}

	case GpuPool:
		if m.dataset.GpuPools == nil {
			data, err = utils.LoadGpuPools(context.Background(), m.repoPath, m.environment)
		}

	case GpuNode:
		if m.dataset.GpuNodeMap == nil {
			data, err = utils.LoadGpuNodes(context.Background(), m.kubeConfig, m.environment)
		}

	case DedicatedAICluster:
		if m.dataset.DedicatedAIClusterMap == nil {
			data, err = utils.LoadDedicatedAIClusters(context.Background(), m.kubeConfig, m.environment)
		}
	}

	if err != nil {
		return errMsg{err}
	}

	return dataMsg{data}
}

func (m *Model) enterContext() tea.Cmd {
	target := m.table.SelectedRow()[0]
	appContext := AppContext{
		Category: m.category,
		Name:     target,
	}
	switch {
	case m.category.IsScope():
		m.context = &appContext
		return m.updateCategory(m.category.ScopedCategories()[0])
	case m.category == Environment:
		env := *utils.FindByName(m.dataset.Environments, target)
		if !m.environment.Equals(env) {
			m.environment = env
			// reset env-bounded data
			m.dataset.BaseModelMap = nil
			m.dataset.GpuPools = nil
			m.dataset.GpuNodeMap = nil
			m.dataset.DedicatedAIClusterMap = nil
			return tea.Batch(
				m.updateCategory(BaseModel),
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
	rows := getTableRows(m.logger, m.dataset, m.category, m.context, m.curFilter)
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
	if m.category == BaseModel {
		if key.Matches(msg, m.keys.ViewModelArtifacts) {
			item := m.getCurrentItem()
			if m.logger != nil {
				m.logger.Info("Viewing model artifacts", zap.Any("item", item))
			}
		}
	}
}

func (m *Model) getCurrentItem() interface{} {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}
