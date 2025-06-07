// Package toolkit implements the core TUI model and logic for the toolkit application.
// It provides the Model struct and related helpers for managing state, events, and rendering
// using Bubble Tea and Charmbracelet components.
package toolkit

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
)

/*
Loader is a composite interface that embeds all loader interfaces.
*/
// Loader interface is now imported from internal/infra/loader.

/*
Model represents the main TUI model for the toolkit application.
It manages state, events, and rendering for the Bubble Tea UI.
*/
type Model struct {
	contextCtx  context.Context
	logger      *zap.Logger
	repoPath    string
	environment models.Environment
	viewHeight  int
	viewWidth   int
	dataset     *models.Dataset
	err         error
	table       *table.Model
	styles      table.Styles
	category    domain.Category
	headers     []header
	target      EditTarget
	mode        StatusMode
	textInput   *textinput.Model
	curFilter   string
	newFilter   string
	chosen      bool
	choice      models.ItemKey
	viewport    *viewport.Model
	renderer    Renderer
	loader      loader.Loader
	reLayout    bool               // layout needs to be updated
	context     *domain.AppContext // selected context
	keys        keyMap
	help        *help.Model
	kubeConfig  string
	// lipgloss styles (moved from package-level for race safety)
	baseStyle      lipgloss.Style
	statusNugget   lipgloss.Style
	statusBarStyle lipgloss.Style
	contextStyle   lipgloss.Style
	statsStyle     lipgloss.Style
	statusText     lipgloss.Style
	infoKeyStyle   lipgloss.Style
	infoValueStyle lipgloss.Style
}

var categoryMap = map[string]domain.Category{
	"t":    domain.Tenant,
	"ld":   domain.LimitDefinition,
	"cpd":  domain.ConsolePropertyDefinition,
	"pd":   domain.PropertyDefinition,
	"lto":  domain.LimitTenancyOverride,
	"cpto": domain.ConsolePropertyTenancyOverride,
	"pto":  domain.PropertyTenancyOverride,
	"cpro": domain.ConsolePropertyRegionalOverride,
	"pro":  domain.PropertyRegionalOverride,
	"bm":   domain.BaseModel,
	"ma":   domain.ModelArtifact,
	"e":    domain.Environment,
	"st":   domain.ServiceTenancy,
	"gp":   domain.GpuPool,
	"gn":   domain.GpuNode,
	"dac":  domain.DedicatedAICluster,
}

/*
NewModel creates a new Model for the toolkit TUI, applying the given options.
*/
func NewModel(opts ...ModelOption) (*Model, error) {
	m := &Model{
		mode:   Normal,
		target: None,
		keys:   keys,
	}

	// Initialize all style fields (previously package-level)
	m.baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	m.statusNugget = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	m.statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})

	m.contextStyle = lipgloss.NewStyle().
		Inherit(m.statusBarStyle).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1).
		MarginRight(1)

	m.statsStyle = m.statusNugget.
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

	m.statusText = lipgloss.NewStyle().Inherit(m.statusBarStyle)
	m.infoKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	m.infoValueStyle = lipgloss.NewStyle()

	// Apply all options
	for _, opt := range opts {
		opt(m)
	}

	// Validation
	if m.repoPath == "" {
		return nil, errors.New("toolkit: repoPath is required")
	}
	if m.environment.Region == "" || m.environment.Type == "" || m.environment.Realm == "" {
		return nil, errors.New("toolkit: environment (Region, Type, Realm) is required")
	}

	if m.loader == nil {
		return nil, errors.New("toolkit: loader is required (use WithLoader option)")
	}
	if m.renderer == nil {
		m.renderer = ProductionRenderer{}
	}
	// Set up defaults if not set by options
	if m.table == nil {
		t := table.New(table.WithFocused(true))
		s := table.DefaultStyles()
		s.Header = s.Header.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true).
			Bold(true)
		s.Selected = s.Selected.
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(false)
		t.SetStyles(s)
		m.table = &t
		m.styles = s
	}
	if m.textInput == nil {
		ti := textinput.New()
		ti.CharLimit = 256
		ti.Prompt = "🐶> "
		m.textInput = &ti
	}
	if m.viewport == nil {
		vp := viewport.New(20, 20)
		vp.Style = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("62"))
		m.viewport = &vp
	}
	if m.help == nil {
		hm := help.New()
		hm.ShowAll = true
		hm.Styles.FullKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))
		hm.Styles.FullDesc = lipgloss.NewStyle()
		m.help = &hm
	}

	return m, nil
}

/*
loggerCtx returns the zap.Logger from the model's field.
*/
func (m *Model) loggerCtx() *zap.Logger {
	if m.logger != nil {
		return m.logger
	}
	return zap.NewNop()
}

// loadData loads the dataset for the current model.
func (m *Model) loadData() tea.Cmd {
	return func() tea.Msg {
		dataset, err := m.loader.LoadDataset(m.contextCtx, m.repoPath, m.environment)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return DataMsg{Data: dataset}
	}
}

// Init implements the tea.Model interface and initializes the model.
func (m *Model) Init() tea.Cmd {
	return m.loadData()
}

// --- Methods moved from model_update.go ---

func (m *Model) updateRows() {
	rows := getTableRows(m.loggerCtx(), m.dataset, m.category, m.context, m.curFilter)
	table.WithRows(rows)(m.table)
	m.table.GotoTop()
}

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

func (m *Model) refreshDisplay() {
	m.curFilter = ""
	m.newFilter = ""
	m.textInput.Reset()
	m.updateColumns()
	m.updateRows()
}

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
	m.refreshDisplay()
}

func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) {
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
	return func() tea.Msg {
		return DataMsg{}
	}
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

func (m *Model) changeCategory() tea.Cmd {
	text := strings.ToLower(m.textInput.Value())
	category, ok := categoryMap[text]
	if !ok {
		return nil
	}
	return m.updateCategory(category)
}

// enterContext moves the model into a new context based on the selected row.
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
		// Defensive: check for FindByName existence
		if m.dataset != nil && len(m.dataset.Environments) > 0 {
			for _, env := range m.dataset.Environments {
				// Use a string representation for matching, e.g., "type/region/realm"
				envKey := strings.ToLower(env.Type + "/" + env.Region + "/" + env.Realm)
				if envKey == strings.ToLower(target) {
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
					break
				}
			}
		}
	default:
		m.enterDetailView()
	}
	return nil
}
