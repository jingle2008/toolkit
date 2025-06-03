// Package toolkit implements the core TUI model and logic for the toolkit application.
// It provides the Model struct and related helpers for managing state, events, and rendering
// using Bubble Tea and Charmbracelet components.
package toolkit

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

type (
	errMsg    struct{ err error }
	dataMsg   struct{ data interface{} }
	filterMsg struct{ text string }
)

/*
Loader is a composite interface that embeds all loader interfaces.
*/
type Loader interface {
	DatasetLoader
	BaseModelLoader
	GpuPoolLoader
	GpuNodeLoader
	DedicatedAIClusterLoader
}

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
	logger      *zap.Logger

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
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to initialize zap logger, using zap.NewNop():", err)
		logger = zap.NewNop()
	}
	m := &Model{
		mode:   Normal,
		target: None,
		keys:   keys,
		logger: logger,
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
