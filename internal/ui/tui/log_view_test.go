package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestFormatLogEntry(t *testing.T) {
	t.Parallel()
	e := logging.Entry{
		Time:    time.Date(2026, 6, 20, 15, 4, 5, 0, time.UTC),
		Level:   logging.LevelInfo,
		Message: "mutation begin",
		Fields:  []any{"action", "drain"},
	}
	assert.Equal(t, "15:04:05 INFO  mutation begin action=drain", formatLogEntry(e))
}

func TestRenderLogEntries_Empty(t *testing.T) {
	t.Parallel()
	m := &Model{logStore: logging.NewRingSink(4)}
	assert.Contains(t, m.renderLogEntries(80), "no log entries")
}

func TestRenderLogEntries_NilStore(t *testing.T) {
	t.Parallel()
	m := &Model{}
	assert.NotPanics(t, func() { _ = m.renderLogEntries(80) })
}

func TestRenderLogEntries_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	ring.Infow("first")
	ring.Errorw("second")
	m := &Model{logStore: ring}
	out := m.renderLogEntries(120)
	assert.Less(t, strings.Index(out, "first"), strings.Index(out, "second"))
	assert.Contains(t, out, "ERROR")
}

func newLogModel(t *testing.T, ring *logging.RingSink) *Model {
	t.Helper()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithCategory(domain.Tenant),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
		WithLogStore(ring),
	)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.viewWidth, m.viewHeight = 80, 12
	return m
}

var backtick = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}}

func TestLogOverlay_ToggleFromList(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.ListView
	_, cmd := m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	assert.Equal(t, common.ListView, m.logReturnView)
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
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.DetailsView, m.viewMode)
}

func TestLogOverlay_PauseAndResume(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 100; i++ {
		ring.Infow("line")
	}
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick) // open → follows
	assert.Contains(t, m.View(), "following")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp}) // scroll up → pause
	assert.Contains(t, m.View(), "PAUSED")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd}) // back to bottom → follow
	assert.Contains(t, m.View(), "following")
}

func TestLogOverlay_TickStopsWhenClosed(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(8))
	m.viewMode = common.ListView // not in LogView
	_, cmd := m.Update(logTickMsg{})
	assert.Nil(t, cmd) // tick does not re-arm outside LogView
}

func TestLogOverlay_NilStoreDoesNotPanic(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.viewWidth, m.viewHeight = 80, 12
	m.viewMode = common.ListView
	assert.NotPanics(t, func() {
		_, _ = m.Update(backtick)
		_ = m.View()
	})
}
