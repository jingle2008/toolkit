package tui

import (
	"fmt"
	"image/color"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/logview"
	"github.com/jingle2008/toolkit/pkg/models"
)

func newLogModel(t *testing.T, ring *logging.RingSink) *Model {
	t.Helper()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithCategory(domain.Tenant),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
		WithLogStore(ring),
	)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.viewWidth, m.viewHeight = 80, 12
	return m
}

var backtick = keyPress("`")

func TestLogOverlay_ToggleFromList(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.ListView
	_, cmd := m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	assert.Equal(t, common.ListView, m.logReturn)
	assert.NotNil(t, cmd) // live-refresh tick started
	// Toggle again closes back to the originating view.
	_, _ = m.Update(backtick)
	assert.Equal(t, common.ListView, m.viewMode)
}

func TestLogOverlay_ReturnsToDetails(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.DetailsView
	_, _ = m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, common.DetailsView, m.viewMode)
}

func TestLogOverlay_QuitKeyQuits(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.ListView
	_, _ = m.Update(backtick)
	_, cmd := m.Update(keyPress("q"))
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

func TestLogOverlay_PauseAndResume(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 100; i++ {
		slog.New(ring).Info("line")
	}
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick) // open → follows
	assert.Contains(t, m.render(), "following")

	_, _ = m.Update(keyPress("pgup")) // scroll up → pause
	assert.Contains(t, m.render(), "PAUSED")

	_, _ = m.Update(keyPress("end")) // back to bottom → follow
	assert.Contains(t, m.render(), "following")
}

// New entries appear on the next tick while following; scrolled up, the
// tick leaves the offset alone.
func TestLogOverlay_TickRefreshesAndFollows(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 50; i++ {
		slog.New(ring).Info("line")
	}
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick)

	slog.New(ring).Info("fresh entry")
	assert.NotContains(t, m.render(), "fresh entry", "View alone does not refresh")
	_, cmd := m.Update(logview.TickMsg{})
	assert.NotNil(t, cmd, "the tick re-arms while open")
	assert.Contains(t, m.render(), "fresh entry")
	assert.Contains(t, m.render(), "following")

	_, _ = m.Update(keyPress("home"))
	slog.New(ring).Info("later entry")
	_, _ = m.Update(logview.TickMsg{})
	vp := m.log.Viewport()
	assert.Equal(t, 0, vp.YOffset(), "paused: the tick keeps the offset")
	assert.Contains(t, m.render(), "PAUSED")
}

func TestLogOverlay_TickStopsWhenClosed(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(8))
	m.viewMode = common.ListView // not in LogView
	_, cmd := m.Update(logview.TickMsg{})
	assert.Nil(t, cmd) // tick does not re-arm outside LogView
}

// A resize re-lays the open log out at the new size.
func TestLogOverlay_ResizeRelaysOut(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	vp := m.log.Viewport()
	assert.Equal(t, 98, vp.Width())
	assert.Equal(t, 20-frameChrome-2, vp.Height())
	assert.Len(t, strings.Split(m.render(), "\n"), 20)
}

// A restyle re-lays the open log out in the new theme's colors.
func TestLogOverlay_RestyleRelaysOut(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	before := m.logView()
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	assert.NotEqual(t, before, m.logView())
}

func TestLogOverlay_NilStoreDoesNotPanic(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	m.viewWidth, m.viewHeight = 80, 12
	m.viewMode = common.ListView
	assert.NotPanics(t, func() {
		_, _ = m.Update(backtick)
		_ = m.render()
	})
	assert.Contains(t, m.render(), "(log store unavailable)")
}

// openLog opens the log over the list with one entry per level.
func openLog(t *testing.T) *Model {
	t.Helper()
	ring := logging.NewRingSink(16)
	slog.New(ring).Debug("probe detail")
	slog.New(ring).Info("loaded tenants")
	slog.New(ring).Warn("slow watch")
	slog.New(ring).Error("drain failed", "node", "n1")
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick)
	return m
}

// / filters as it is typed; enter keeps the filter, then esc clears the
// filters before a second esc closes the log.
func TestLogView_FilterThenEsc(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	m.updateLogView(keyPress("/"))
	require.True(t, m.log.Typing())
	for _, r := range "DRAIN" {
		m.updateLogView(keyPress(string(r)))
	}
	shown, _ := m.log.Counts()
	assert.Equal(t, 1, shown, "case-insensitive")
	assert.Contains(t, m.logView(), "node=n1")

	m.updateLogView(keyPress("enter"))
	assert.False(t, m.log.Typing())
	assert.Equal(t, "DRAIN", m.log.Filter())
	assert.Contains(t, ansi.Strip(m.logView()), "⟨/DRAIN⟩")

	m.updateLogView(keyPress("esc"))
	assert.Empty(t, m.log.Filter())
	assert.Equal(t, common.LogView, m.viewMode, "the first esc clears the filter")
	m.updateLogView(keyPress("esc"))
	assert.Equal(t, common.ListView, m.viewMode)
}

// q and ` are text while typing a filter.
func TestLogView_TypingTakesKeysAsText(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	m.updateLogView(keyPress("/"))
	m.updateLogView(keyPress("q"))
	m.updateLogView(keyPress("`"))
	assert.Equal(t, "q`", m.log.Filter())
	assert.Equal(t, common.LogView, m.viewMode)
}

// The log sits in the frame: env bar, breadcrumb, a titled panel and its
// keys.
func TestLogView_InFrame(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	out := ansi.Strip(m.logView())
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 20)
	assert.Contains(t, lines[1], "Log")
	assert.True(t, strings.HasPrefix(lines[2], "╭───"), "the breadcrumb names the log: %s", lines[2])
	assert.Contains(t, lines[len(lines)-2], " 4 entries ─╯", "the bottom edge counts the entries")
	assert.Contains(t, lines[len(lines)-1], "/ filter  l level")
}

// Rendering the log changes nothing: two renders leave the model's log
// state equal.
func TestLogView_RenderHasNoSideEffects(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 100; i++ {
		slog.New(ring).Info("line")
	}
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick)
	slog.New(ring).Info("unseen until the tick")
	m.viewWidth, m.viewHeight = 60, 30 // a size the log was not laid out at

	// %#v, not reflect.DeepEqual: the viewport holds a func, which
	// DeepEqual never finds equal.
	state := func() string { return fmt.Sprintf("%#v", m.log) }
	before := state()
	first := m.render()
	assert.Equal(t, before, state())
	assert.Equal(t, first, m.render())
	assert.Equal(t, before, state())
	assert.NotContains(t, first, "unseen until the tick")
}
