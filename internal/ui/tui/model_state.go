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

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/recentenv"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/internal/uistate"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// watchState holds the live-update state for the two watches that drive
// the status-bar "● LIVE" indicator and trigger background reloads.
type watchState struct {
	// k8sActive is true while a live k8s watch is active for the current
	// category. Reset on every category change and cleared on watch fallback.
	k8sActive bool

	// k8sTrigger is the active category's trigger channel; held so a
	// k8sWatchTriggeredMsg can re-arm the listener on the same stream.
	k8sTrigger <-chan struct{}

	// repoTrigger is the live working-tree trigger channel; nil when the
	// repo watch is unavailable. repoActive is true while it is established.
	repoTrigger <-chan struct{}
	repoActive  bool
}

// logOverlay holds the state for the log-viewer overlay (toggled with the
// log keybinding); returnView restores the prior view when it closes.
type logOverlay struct {
	store      *logging.RingSink
	viewport   *viewport.Model
	returnView common.ViewMode
}

// toastManager holds the transient banners shown over the active view.
// active is the newest, nil when no toast is showing; older holds up to
// maxStackedToasts-1 earlier ones still alive, oldest first. seq is a
// monotonic id source that persists across toasts so toastExpireMsg can
// match the one it was scheduled for.
type toastManager struct {
	active *toastState
	older  []toastState
	seq    int
}

// genCounters holds monotonic generation counters used to discard stale
// async responses. The invariant: before dispatching async work, the
// relevant counter is bumped and its value captured in the outgoing
// command; when the result arrives it carries that captured generation,
// and the handler drops it unless it still matches the current counter.
// Each counter guards an independent pipeline:
//   - msg:    category loads and live-watch messages (the main dataset)
//   - filter: list filtering
//   - rows:   table-row recomputation
//   - detail: detail-view rendering
type genCounters struct {
	msg    int
	filter int
	rows   int
	detail int
}

// next* advance a counter and return the new generation in one step, so a
// caller cannot accidentally capture a stale value or bump without
// capturing. Called as a statement (ignoring the result) they serve as a
// pure "invalidate in-flight work" bump.
func (g *genCounters) nextMsg() int    { g.msg++; return g.msg }
func (g *genCounters) nextFilter() int { g.filter++; return g.filter }
func (g *genCounters) nextRows() int   { g.rows++; return g.rows }
func (g *genCounters) nextDetail() int { g.detail++; return g.detail }

/*
Model represents the main TUI model for the toolkit application.
It manages state, events, and rendering for the Bubble Tea UI.
*/
type Model struct {
	pendingTasks  int
	logger        logging.Logger
	parentCtx     context.Context //nolint:containedctx // stored to manage lifecycle across async loads
	loadCtx       context.Context //nolint:containedctx // stored to manage lifecycle across async loads
	loadCancel    context.CancelFunc
	repoPath      string
	environment   models.Environment
	viewHeight    int
	viewWidth     int
	dataset       *models.Dataset
	table         *table.Model
	styles        table.Styles
	category      domain.Category
	headers       []header
	editTarget    common.EditTarget
	inputMode     common.InputMode
	textInput     *textinput.Model
	filter        string
	initialFilter string
	viewMode      common.ViewMode
	lastViewMode  common.ViewMode // for toggling help view
	helpScroll    int             // first help body line shown when help scrolls
	prefix        string          // pending prefix key (keys.SortLeader) or ""
	confirm       confirmOverlay  // destructive-action confirmation modal state
	rowPicker     rowPickerState  // modal row picker state (go to, metrics DAC)
	palette       paletteState    // command palette state
	preview       bool            // preview pane toggled on (shown when wide enough)
	uiState       uistate.State   // persisted UI preferences
	uiStatePath   string          // where uiState is saved ("" disables saving and the hint)
	keysHint      bool            // the keys-changed hint is showing
	selectedKey   models.ItemKey
	viewport      *viewport.Model
	renderer      view.Renderer
	loader        loader.Composite
	scope         *domain.Scope // selected scope (parent context for current category)
	keys          keys.KeyMap
	help          *help.Model
	kubeConfig    string
	version       string
	k8sVersion    string // API server version for the current kube context; "" until fetched
	// theme holds the app-level lipgloss styles (status bar, info pane,
	// help view). Set once via setStyles; see the Styles struct in styles.go.
	theme Styles
	stats tableStats

	// Spinner for loading screen
	loadingSpinner *spinner.Model

	// Stopwatch for loading duration
	loadingTimer *stopwatch.Model

	// gens holds the generation counters that guard against stale async
	// responses. See the genCounters type.
	gens genCounters

	// watch holds live-update state for the k8s cluster watch and the
	// repo working-tree watch. See the watchState type.
	watch watchState

	// Table sorting state
	sortColumn string
	sortAsc    bool

	// recentEnvs holds the environments bound to the 0–5 quick-switch
	// keys; recentEnvsPath is where they persist ("" disables saving).
	recentEnvs     *recentenv.List
	recentEnvsPath string
	recentSaver    recentSaver

	// Category navigation history
	history    []domain.Category // chronological list of visited categories
	historyIdx int               // index of the current position in history

	// Show only faulty items in list view: rows whose model reports
	// IsFaulty, or that name a parent row that doesn't exist (see
	// faultyFilter); the gate itself is category-agnostic.
	showFaulty bool

	// rawRows mirrors m.table.Rows() pre-truncation so itemKeyFrom can
	// recover un-elided Name/Tenant cells. applyMiddleTruncation
	// mutates the table's rows in place, which would otherwise leak
	// "…" into ScopedItemKey lookups.
	rawRows []table.Row

	// Export CSV popup state
	dirPicker *filepicker.Model

	// Tenant-metadata entry form state (EditTenantView).
	editTenant *editTenantForm

	// log holds the log-overlay state. See the logOverlay type.
	log logOverlay

	// toasts holds the transient banner shown over the active view. See
	// the toastManager type.
	toasts toastManager
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

// initStyles initializes all style fields for the model using shared style
// definitions. It assumes a dark background until the terminal reports its
// color (tea.BackgroundColorMsg, requested in Init).
func initStyles(m *Model) {
	setStyles(m, DefaultStyles(true))
}

// setStyles makes s the model's theme and pushes its component styles
// into whichever bubbles exist yet; setDefaults calls it again once they
// all do, and a tea.BackgroundColorMsg restyles everything through it.
func setStyles(m *Model, s Styles) {
	m.theme = s
	if m.table != nil {
		m.table.SetStyles(s.Table)
		m.styles = s.Table
	}
	if m.viewport != nil {
		m.viewport.Style = s.Viewport
	}
	if m.help != nil {
		m.help.Styles.FullKey = s.HelpKey
		m.help.Styles.FullDesc = s.HelpDesc
		m.help.Styles.ShortKey = s.HelpKey
		m.help.Styles.ShortDesc = s.HelpDesc
	}
	if m.loadingSpinner != nil {
		m.loadingSpinner.Style = s.Spinner
	}
}

// applyOptions applies all ModelOption functions to the model.
func applyOptions(m *Model, opts []ModelOption) {
	for _, opt := range opts {
		opt(m)
	}
}

// newLoadContext cancels any in-flight load and creates a fresh context for the next load.
func (m *Model) newLoadContext() {
	if m.loadCancel != nil {
		// Cancel any in-flight load to prevent stale work
		m.loadCancel()
		if m.logger != nil {
			m.logger.Infow("canceled in-flight load")
		}
	}
	parent := m.parentCtx
	if parent == nil {
		parent = context.Background()
	}
	m.loadCtx, m.loadCancel = context.WithCancel(parent)
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
		m.table = &t
	}
	if m.textInput == nil {
		ti := textinput.New()
		ti.CharLimit = 256
		ti.Prompt = " 🐶> "
		m.textInput = &ti
	}
	if m.viewport == nil {
		vp := viewport.New(viewport.WithWidth(20), viewport.WithHeight(20))
		disableHorizontalScroll(&vp)
		m.viewport = &vp
	}
	if m.log.viewport == nil {
		lvp := viewport.New(viewport.WithWidth(20), viewport.WithHeight(20))
		disableHorizontalScroll(&lvp)
		m.log.viewport = &lvp
	}
	if m.recentEnvs == nil {
		m.recentEnvs = &recentenv.List{}
	}
	if m.help == nil {
		hm := help.New()
		hm.ShowAll = true
		m.help = &hm
	}
	if m.loadingSpinner == nil {
		loadingSpinner := spinner.New(spinner.WithSpinner(spinner.Points))
		m.loadingSpinner = &loadingSpinner
	}
	if m.loadingTimer == nil {
		sw := stopwatch.New(stopwatch.WithInterval(time.Second))
		m.loadingTimer = &sw
	}
	if m.dirPicker == nil {
		homeDir, _ := os.UserHomeDir()
		p := filepicker.New()
		p.CurrentDirectory = homeDir
		p.DirAllowed = true
		p.FileAllowed = false
		p.SetHeight(15)
		// Release esc so updateExportView can use it to dismiss the popup;
		// h/backspace/left still go up a directory.
		p.KeyMap.Back.SetKeys("h", "backspace", "left")
		m.dirPicker = &p
	}
	setStyles(m, m.theme)
}

// cancelInFlight cancels any in-flight async operations (loads, actions).
func (m *Model) cancelInFlight() {
	if m.loadCancel != nil {
		m.loadCancel()
		if m.logger != nil {
			m.logger.Infow("canceled in-flight tasks")
		}
	}
}

// sessionCtx returns the session-scoped context (survives navigation, cancels
// on shutdown) used by the always-on repo watch and its background reloads.
func (m *Model) sessionCtx() context.Context {
	if m.parentCtx == nil {
		return context.Background()
	}
	return m.parentCtx
}

// opCtx returns a 30s-timeout context for a one-shot action (cordon,
// drain, scale, mutate). It derives from m.parentCtx so the action cancels
// when the TUI shuts down, but unlike m.loadCtx it survives navigation /
// refresh so a user pressing 'r' mid-cordon doesn't abort the cordon.
func (m *Model) opCtx() (context.Context, context.CancelFunc) {
	parent := m.parentCtx
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 30*time.Second)
}

// longOpCtx returns the context for a long-running workflow (e.g. DAC deletion,
// which deletes endpoints then polls a work request for minutes). It imposes no
// 30s cap — the workflow's own internal timeout governs duration — but still
// cancels on TUI shutdown and survives navigation/refresh.
func (m *Model) longOpCtx() context.Context {
	return m.sessionCtx()
}

// disableHorizontalScroll unbinds the v2 viewport's left/right keys. They
// default to h/l, and h is Help; the details and log bodies wrap, so
// there is nothing to scroll sideways.
func disableHorizontalScroll(vp *viewport.Model) {
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
}
