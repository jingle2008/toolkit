// Package toolkit implements the core TUI model and logic for the toolkit application.
// It provides the Model struct and related helpers for managing state, events, and rendering
// using Bubble Tea and Charmbracelet components.
package toolkit

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

type (
	errMsg    struct{ err error }
	dataMsg   struct{ data interface{} }
	filterMsg struct{ text string }
)

// Model represents the main TUI model for the toolkit application.
type Model struct {
	repoPath    string
	environment models.Environment
	viewHeight  int
	viewWidth   int
	dataset     *models.Dataset
	err         error
	table       *table.Model
	styles      table.Styles
	category    Category
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
	loader      Loader
	reLayout    bool        // layout needs to be updated
	context     *AppContext // selected context
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

var categoryMap = map[string]Category{
	"t":    Tenant,
	"ld":   LimitDefinition,
	"cpd":  ConsolePropertyDefinition,
	"pd":   PropertyDefinition,
	"lto":  LimitTenancyOverride,
	"cpto": ConsolePropertyTenancyOverride,
	"pto":  PropertyTenancyOverride,
	"cpro": ConsolePropertyRegionalOverride,
	"pro":  PropertyRegionalOverride,
	"bm":   BaseModel,
	"ma":   ModelArtifact,
	"e":    Environment,
	"st":   ServiceTenancy,
	"gp":   GpuPool,
	"gn":   GpuNode,
	"dac":  DedicatedAICluster,
}

/*
NewModel creates a new Model for the toolkit TUI, applying the given options.
*/
func NewModel(opts ...ModelOption) *Model {
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

	if m.loader == nil {
		m.loader = ProductionLoader{}
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

	return m
}

// loadData loads the dataset for the current model.
func (m *Model) loadData() tea.Cmd {
	return func() tea.Msg {
		dataset, err := utils.LoadDataset(m.repoPath, m.environment)
		if err != nil {
			return errMsg{err}
		}
		return dataMsg{dataset}
	}
}

// Init implements the tea.Model interface and initializes the model.
func (m *Model) Init() tea.Cmd {
	return m.loadData()
}

// Update handles incoming messages and updates the model state.
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

func (m *Model) getCurrentItem() interface{} {
	key := getItemKey(m.category, m.table.SelectedRow())
	return findItem(m.dataset, m.category, key)
}

func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) {
	//nolint:exhaustive
	if m.category == BaseModel {
		if key.Matches(msg, m.keys.ViewModelArtifacts) {
			item := m.getCurrentItem()
			log.Printf("Viewing model artifacts for %s\n", item)
		}
	}
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
			data, err = utils.LoadBaseModels(m.repoPath, m.environment)
		}

	case GpuPool:
		if m.dataset.GpuPools == nil {
			data, err = utils.LoadGpuPools(m.repoPath, m.environment)
		}

	case GpuNode:
		if m.dataset.GpuNodeMap == nil {
			data, err = utils.LoadGpuNodes(m.kubeConfig, m.environment)
		}

	case DedicatedAICluster:
		if m.dataset.DedicatedAIClusterMap == nil {
			data, err = utils.LoadDedicatedAIClusters(m.kubeConfig, m.environment)
		}
	}

	if err != nil {
		return errMsg{err}
	}

	return dataMsg{data}
}

func (m *Model) refreshDisplay() {
	m.curFilter = ""    // reset current filter
	m.newFilter = ""    // ignore future filter message
	m.textInput.Reset() // reset filter display

	m.updateColumns()
	m.updateRows()
}

func (m *Model) updateContent(width int) {
	if !m.chosen {
		return
	}

	var err error
	item := findItem(m.dataset, m.category, m.choice)
	content, err := utils.PrettyJSON(item)
	if err != nil {
		content = err.Error()
	}
	str, err := m.renderer.RenderJSON(content, width)
	if err != nil {
		wrappedErr := fmt.Errorf("error encountered rendering content: %w", err)
		log.Println(wrappedErr)
		return
	}
	m.viewport.SetContent(str)
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
	rows := getTableRows(m.dataset, m.category, m.context, m.curFilter)
	table.WithRows(rows)(m.table)

	m.table.GotoTop()
}

func (m *Model) debounceFilter() tea.Cmd {
	m.newFilter = strings.ToLower(m.textInput.Value())

	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return filterMsg{m.newFilter}
	})
}

func (m *Model) changeCategory() tea.Cmd {
	text := strings.ToLower(m.textInput.Value())

	category, ok := categoryMap[text]
	if !ok {
		return nil
	}

	return m.updateCategory(category)
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

func centerText(text string, width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(text)
}

func (m *Model) infoView() string {
	keys := []string{"Realm:", "Type:", "Region:"}
	values := []string{m.environment.Realm, m.environment.Type, m.environment.Region}

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.infoKeyStyle.Render(strings.Join(keys, "\n")),
		" ",
		m.infoValueStyle.Render(strings.Join(values, "\n")),
	)

	return content
}

func (m *Model) contextString() string {
	scope := "all"
	if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		scope = m.context.Name
	}

	if m.chosen {
		keyString := getItemKeyString(m.category, m.choice)
		scope = fmt.Sprintf("%s/%s", scope, keyString)
	}

	return fmt.Sprintf("%s (%s)", m.category.String(), scope)
}

func (m *Model) statusView() string {
	w := lipgloss.Width

	contextCell := m.contextStyle.Render(m.contextString())

	statsCell := m.statsStyle.Render(
		fmt.Sprintf("[%d/%d]", m.table.Cursor()+1, len(m.table.Rows())))
	inputCell := m.statusText.
		Width(m.viewWidth - w(contextCell) - w(statsCell)).
		Render(m.textInput.View())

	return lipgloss.JoinHorizontal(lipgloss.Top,
		contextCell,
		inputCell,
		statsCell,
	)
}

// View renders the current state of the model as a string.
func (m *Model) View() string {
	if m.err != nil {
		return centerText(m.err.Error(), m.viewWidth, m.viewHeight)
	}

	helpView := m.help.View(m.keys)
	infoView := m.infoValueStyle.
		Width(m.viewWidth - lipgloss.Width(helpView)).Render(m.infoView())
	header := lipgloss.JoinHorizontal(lipgloss.Top, infoView, helpView)

	var mainContent string
	if !m.chosen {
		mainContent = m.baseStyle.Render(m.table.View())
	} else {
		mainContent = m.viewport.View()
	}

	status := m.statusView()

	return lipgloss.JoinVertical(lipgloss.Left, header, status, mainContent)
}
