/*
Package tui contains the Model struct and constructor for the toolkit TUI.
This file defines the main state container and its initialization logic.
*/
package tui

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
Model represents the main TUI model for the toolkit application.
It manages state, events, and rendering for the Bubble Tea UI.
*/
type Model struct {
	pendingTasks   int
	logger         logging.Logger
	ctx            context.Context //nolint:containedctx
	repoPath       string
	environment    models.Environment
	viewHeight     int
	viewWidth      int
	dataset        *models.Dataset
	err            error
	table          *table.Model
	styles         table.Styles
	category       domain.Category
	headers        []header
	editTarget     common.EditTarget
	inputMode      common.InputMode
	textInput      *textinput.Model
	curFilter      string
	newFilter      string
	viewMode       common.ViewMode
	lastViewMode   common.ViewMode // for toggling help view
	choice         models.ItemKey
	viewport       *viewport.Model
	renderer       view.Renderer
	loader         loader.Loader
	context        *domain.ToolkitContext // selected context
	keys           keys.KeyMap
	help           *help.Model
	kubeConfig     string
	version        string
	baseStyle      lipgloss.Style
	statusNugget   lipgloss.Style
	statusBarStyle lipgloss.Style
	contextStyle   lipgloss.Style
	statsStyle     lipgloss.Style
	statusText     lipgloss.Style
	infoKeyStyle   lipgloss.Style
	infoValueStyle lipgloss.Style
	stats          tableStats

	// Help view styles
	helpBorder lipgloss.Style
	helpHeader lipgloss.Style
	helpKey    lipgloss.Style
	helpDesc   lipgloss.Style

	// Spinner for loading screen
	loadingSpinner *spinner.Model

	// Stopwatch for loading duration
	loadingTimer *stopwatch.Model

	// Message generation to guard against stale async responses
	gen int

	// Table sorting state
	sortColumn string
	sortAsc    bool

	// Category navigation history
	history    []domain.Category // chronological list of visited categories
	historyIdx int               // index of the current position in history

	// Show only faulty items in list view (Tenant, GpuNode, DedicatedAICluster)
	showFaulty bool

	// Export CSV popup state
	dirPicker *filepicker.Model
}

/*
NewModel creates a new Model for the toolkit TUI, applying the given options.
*/
func NewModel(opts ...ModelOption) (*Model, error) {
	m := &Model{
		inputMode:    common.NormalInput,
		editTarget:   common.NoneTarget,
		category:     domain.Tenant, // or a sensible default
		viewMode:     common.ListView,
		lastViewMode: common.ListView,
		sortColumn:   common.NameCol,
		sortAsc:      true,
		history:      []domain.Category{},
		historyIdx:   -1,
	}

	initStyles(m)
	applyOptions(m, opts)
	if err := validateModel(m); err != nil {
		return nil, err
	}
	setDefaults(m)

	// Seed initial category in history if not already present
	if len(m.history) == 0 {
		m.history = []domain.Category{m.category}
		m.historyIdx = 0
	}

	// Initialize keys based on initial category and mode
	m.keys = keys.ResolveKeys(m.category, m.viewMode)

	return m, nil
}

// initStyles initializes all style fields for the model using shared style definitions.
func initStyles(m *Model) {
	s := DefaultStyles()

	m.baseStyle = s.Base
	m.statusNugget = s.StatusNugget
	m.statusBarStyle = s.StatusBar
	m.contextStyle = s.Context
	m.statsStyle = s.Stats
	m.statusText = s.StatusText
	m.infoKeyStyle = s.InfoKey
	m.infoValueStyle = s.InfoValue

	// Help view styles
	m.helpBorder = s.HelpBorder
	m.helpHeader = s.HelpHeader
	m.helpKey = s.HelpKey
	m.helpDesc = s.HelpDesc
}

// applyOptions applies all ModelOption functions to the model.
func applyOptions(m *Model, opts []ModelOption) {
	for _, opt := range opts {
		opt(m)
	}
}

// validateModel checks required fields and returns an error if any are missing.
func validateModel(m *Model) error {
	if m.repoPath == "" {
		return errors.New("toolkit: repoPath is required")
	}
	if m.environment.Region == "" || m.environment.Type == "" || m.environment.Realm == "" {
		return errors.New("toolkit: environment (Region, Type, Realm) is required")
	}
	if m.loader == nil {
		return errors.New("toolkit: loader is required (use WithLoader option)")
	}
	if m.logger == nil {
		return errors.New("toolkit: logger is required (use WithLogger option)")
	}
	return nil
}

// setDefaults sets up default values for fields if not set by options.
func setDefaults(m *Model) {
	if m.renderer == nil {
		m.renderer = view.ProductionRenderer{}
	}
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
		ti.Prompt = " 🐶> "
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
		keyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))
		descStyle := lipgloss.NewStyle()
		hm := help.New()
		hm.ShowAll = true
		hm.Styles.FullKey = keyStyle
		hm.Styles.FullDesc = descStyle
		hm.Styles.ShortKey = keyStyle
		hm.Styles.ShortDesc = descStyle
		m.help = &hm
	}
	if m.loadingSpinner == nil {
		loadingSpinner := spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))),
		)
		m.loadingSpinner = &loadingSpinner
	}
	if m.loadingTimer == nil {
		sw := stopwatch.NewWithInterval(time.Millisecond * 500)
		m.loadingTimer = &sw
	}
	if m.dirPicker == nil {
		homeDir, _ := os.UserHomeDir()
		p := filepicker.New()
		p.CurrentDirectory = homeDir
		p.DirAllowed = true
		p.FileAllowed = false
		p.SetHeight(15)
		m.dirPicker = &p
	}
}
