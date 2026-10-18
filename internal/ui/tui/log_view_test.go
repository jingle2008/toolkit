package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	m := &Model{log: logOverlay{store: logging.NewRingSink(4)}}
	body, _, _ := m.renderLogEntries(80)
	assert.Contains(t, body, "no log entries")
}

func TestRenderLogEntries_NilStore(t *testing.T) {
	t.Parallel()
	m := &Model{}
	assert.NotPanics(t, func() { _, _, _ = m.renderLogEntries(80) })
}

func TestRenderLogEntries_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	ring.Infow("first")
	ring.Errorw("second")
	m := &Model{log: logOverlay{store: ring}}
	out, _, _ := m.renderLogEntries(120)
	assert.Less(t, strings.Index(out, "first"), strings.Index(out, "second"))
	assert.Contains(t, out, "ERROR")
}

// A log entry wider than the view wraps to multiple lines instead of being
// truncated with an ellipsis — the whole message stays readable.
func TestRenderLogEntries_WrapsInsteadOfTruncating(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(4)
	long := strings.Repeat("x", 100) // one unbroken token, no color (Info)
	ring.Infow(long)
	m := &Model{log: logOverlay{store: ring}}

	out, _, _ := m.renderLogEntries(40)

	assert.NotContains(t, out, "…", "entries should wrap, not truncate with an ellipsis")
	assert.Contains(t, out, "\n", "a 100-char entry must wrap to multiple lines at width 40")
	assert.GreaterOrEqual(t, strings.Count(out, "x"), 100, "the full message must survive wrapping")
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

var backtick = keyPress("`")

func TestLogOverlay_ToggleFromList(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.ListView
	_, cmd := m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	assert.Equal(t, common.ListView, m.log.returnView)
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

func TestLogOverlay_PauseAndResume(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 100; i++ {
		ring.Infow("line")
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
		_ = m.render()
	})
}

// openLog opens the log over the list with one entry per level.
func openLog(t *testing.T) *Model {
	t.Helper()
	ring := logging.NewRingSink(16)
	ring.Debugw("probe detail")
	ring.Infow("loaded tenants")
	ring.Warnw("slow watch")
	ring.Errorw("drain failed", "node", "n1")
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick)
	return m
}

// l cycles the minimum level all → info → warn → error → all.
func TestLogView_LevelCycles(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	for _, want := range []int{4, 3, 2, 1, 4} {
		_, shown, total := m.renderLogEntries(80)
		assert.Equal(t, want, shown)
		assert.Equal(t, 4, total)
		m.updateLogView(keyPress("l"))
	}
}

// / filters as it is typed; enter keeps the filter, then esc clears the
// filters before a second esc closes the log.
func TestLogView_FilterThenEsc(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	m.updateLogView(keyPress("/"))
	require.True(t, m.log.typing)
	for _, r := range "DRAIN" {
		m.updateLogView(keyPress(string(r)))
	}
	body, shown, _ := m.renderLogEntries(80)
	assert.Equal(t, 1, shown, "case-insensitive")
	assert.Contains(t, body, "node=n1")

	m.updateLogView(keyPress("enter"))
	assert.False(t, m.log.typing)
	assert.Equal(t, "DRAIN", m.log.filter)
	assert.Contains(t, ansi.Strip(m.logCrumb(1, 4)), "⟨/DRAIN⟩")

	m.updateLogView(keyPress("esc"))
	assert.Empty(t, m.log.filter)
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
	assert.Equal(t, "q`", m.log.filter)
	assert.Equal(t, common.LogView, m.viewMode)
}

// A filter that hides everything says so and how to undo it.
func TestLogView_NoMatches(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	m.log.filter = "zzz"
	body, shown, total := m.renderLogEntries(80)
	assert.Equal(t, 0, shown)
	assert.Equal(t, 4, total)
	assert.Contains(t, body, "no entries match · esc clears the filters")
}

// The log sits in the frame: env bar, breadcrumb, a titled panel and its
// keys.
func TestLogView_InFrame(t *testing.T) {
	t.Parallel()
	m := openLog(t)
	m.viewWidth, m.viewHeight = 100, 20
	out := ansi.Strip(m.logView())
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 20)
	assert.Contains(t, lines[1], "Log")
	assert.Contains(t, lines[1], "[4/4]")
	assert.True(t, strings.HasPrefix(lines[2], "╭─ Log (4) "), lines[2])
	assert.Contains(t, lines[len(lines)-1], "/ filter  l level")
}
