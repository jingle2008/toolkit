/*
Package tui contains the Model struct and constructor for the toolkit TUI.
This file defines the main state container and its initialization logic.
*/
package tui

import (
	"context"
	"errors"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
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
	reLayout       bool                   // layout needs to be updated
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

	// Help view styles
	helpBorder lipgloss.Style
	helpHeader lipgloss.Style
	helpKey    lipgloss.Style
	helpDesc   lipgloss.Style

	// Spinner for loading screen
	loadingSpinner *spinner.Model

	// Table sorting state
	sortColumn string
	sortAsc    bool
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
		sortColumn:   "Name",
		sortAsc:      true,
	}

	initStyles(m)
	applyOptions(m, opts)
	if err := validateModel(m); err != nil {
		return nil, err
	}
	setDefaults(m)

	// Initialize keys based on initial category and mode
	m.keys = keys.ResolveKeys(m.category, m.viewMode)

	return m, nil
}

// initStyles initializes all style fields for the model.
func initStyles(m *Model) {
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
	m.infoValueStyle = lipgloss.NewStyle().Width(30)

	// Help view styles
	m.helpBorder = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)
	m.helpHeader = lipgloss.NewStyle().Inherit(m.infoKeyStyle).Underline(true)
	m.helpKey = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	m.helpDesc = lipgloss.NewStyle()
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

	if m.loadingSpinner == nil {
		loadingSpinner := spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))),
		)
		m.loadingSpinner = &loadingSpinner
	}
}
