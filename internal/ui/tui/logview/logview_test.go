package logview

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logging "github.com/jingle2008/toolkit/internal/logging"
)

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

var styles = Styles{
	Text: lipgloss.Color("7"), Accent: lipgloss.Color("4"), Muted: lipgloss.Color("8"), Primary: lipgloss.Color("5"),
	Warn: lipgloss.NewStyle().Foreground(lipgloss.Color("3")), Error: lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
	Debug: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
}

// open lays a log over ring out at 80×7, as the parent does on open.
func open(ring *logging.RingSink) Model {
	return New(ring).Refresh(80, 7, styles)
}

// fourLevels is a log with one entry per level.
func fourLevels() Model {
	ring := logging.NewRingSink(16)
	slog.New(ring).Debug("probe detail")
	slog.New(ring).Info("loaded tenants")
	slog.New(ring).Warn("slow watch")
	slog.New(ring).Error("drain failed", "node", "n1")
	return open(ring)
}

// send feeds keys to m and returns the last Result.
func send(m Model, ks ...string) (Model, Result) {
	var res Result
	for _, k := range ks {
		m, res, _ = m.Update(press(k))
	}
	return m, res
}

func many(n int) *logging.RingSink {
	ring := logging.NewRingSink(n + 50)
	for i := range n {
		slog.New(ring).Info(fmt.Sprintf("line %d", i))
	}
	return ring
}

func TestFormatEntry(t *testing.T) {
	t.Parallel()
	e := logging.Entry{
		Time:    time.Date(2026, 6, 20, 15, 4, 5, 0, time.UTC),
		Level:   logging.LevelInfo,
		Message: "mutation begin",
		Fields:  []any{"action", "drain"},
	}
	assert.Equal(t, "15:04:05 INFO  mutation begin action=drain", FormatEntry(e))
}

func TestLevelLabel(t *testing.T) {
	t.Parallel()
	for l, want := range map[logging.Level]string{
		logging.LevelDebug: "DEBUG", logging.LevelInfo: "INFO", logging.LevelWarn: "WARN",
		logging.LevelError: "ERROR", logging.Level(42): "?",
	} {
		assert.Equal(t, want, LevelLabel(l))
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	assert.False(t, Model{}.Ready())
	ring := logging.NewRingSink(4)
	m := New(ring)
	assert.True(t, m.Ready())
	assert.Same(t, ring, m.Store())
	assert.Equal(t, logging.LevelDebug, m.MinLevel())
	vp := m.Viewport()
	assert.False(t, vp.KeyMap.Left.Enabled() || vp.KeyMap.Right.Enabled(), "h is the app's Help")
}

func TestRender_Empty(t *testing.T) {
	t.Parallel()
	body, _, _ := open(logging.NewRingSink(4)).render(80)
	assert.Contains(t, body, "no log entries")
}

func TestRender_NilStore(t *testing.T) {
	t.Parallel()
	m := open(nil)
	assert.Contains(t, ansi.Strip(m.View()), "(log store unavailable)")
}

func TestRender_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	slog.New(ring).Info("first")
	slog.New(ring).Error("second")
	out, _, _ := open(ring).render(120)
	assert.Less(t, strings.Index(out, "first"), strings.Index(out, "second"))
	assert.Contains(t, out, "ERROR")
}

// A log entry wider than the view wraps to multiple lines instead of being
// truncated with an ellipsis — the whole message stays readable.
func TestRender_WrapsInsteadOfTruncating(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(4)
	slog.New(ring).Info(strings.Repeat("x", 100)) // one unbroken token
	out, _, _ := open(ring).render(40)
	assert.NotContains(t, out, "…", "entries should wrap, not truncate with an ellipsis")
	assert.Contains(t, out, "\n", "a 100-char entry must wrap to multiple lines at width 40")
	assert.GreaterOrEqual(t, strings.Count(out, "x"), 100, "the full message must survive wrapping")
}

// l cycles the minimum level all → info → warn → error → all.
func TestUpdate_LevelCycles(t *testing.T) {
	t.Parallel()
	m := fourLevels()
	for _, want := range []int{4, 3, 2, 1, 4} {
		shown, total := m.Counts()
		assert.Equal(t, want, shown)
		assert.Equal(t, 4, total)
		m, _ = send(m, "l")
	}
}

func TestView_LevelChip(t *testing.T) {
	t.Parallel()
	m, _ := send(fourLevels(), "l", "l")
	assert.Contains(t, ansi.Strip(m.View()), "⟨≥warn⟩")
	assert.Contains(t, ansi.Strip(m.View()), "2 of 4")
}

// / filters as it is typed; enter keeps the filter, then esc clears the
// filters before a second esc closes.
func TestUpdate_FilterThenEsc(t *testing.T) {
	t.Parallel()
	m, _ := send(fourLevels(), "/")
	require.True(t, m.Typing())
	assert.Contains(t, ansi.Strip(m.View()), "enter apply")
	m, _ = send(m, "D", "R", "A", "I", "N")
	shown, _ := m.Counts()
	assert.Equal(t, 1, shown, "case-insensitive")
	assert.Contains(t, m.View(), "node=n1")

	m, _ = send(m, "enter")
	assert.False(t, m.Typing())
	assert.Equal(t, "DRAIN", m.Filter())
	assert.Contains(t, ansi.Strip(m.crumb()), "⟨/DRAIN⟩")

	m, res := send(m, "esc")
	assert.Empty(t, m.Filter())
	assert.Equal(t, None, res.Outcome, "the first esc clears the filter")
	shown, _ = m.Counts()
	assert.Equal(t, 4, shown)
	_, res = send(m, "esc")
	assert.Equal(t, Closed, res.Outcome)
}

// esc in the input drops the filter typed so far.
func TestUpdate_EscInInputClears(t *testing.T) {
	t.Parallel()
	m, _ := send(fourLevels(), "/", "s", "l", "o", "w")
	shown, _ := m.Counts()
	assert.Equal(t, 1, shown)
	m, res := send(m, "esc")
	assert.Equal(t, None, res.Outcome)
	assert.False(t, m.Typing())
	assert.Empty(t, m.Filter())
	shown, _ = m.Counts()
	assert.Equal(t, 4, shown)
}

// q and ` are text while typing; ctrl+c still quits.
func TestUpdate_TypingTakesKeysAsText(t *testing.T) {
	t.Parallel()
	m, res := send(fourLevels(), "/", "q", "`")
	assert.Equal(t, "q`", m.Filter())
	assert.Equal(t, None, res.Outcome)
	_, res = send(m, "ctrl+c")
	assert.Equal(t, Quit, res.Outcome)
}

func TestUpdate_PasteTypesWhileTyping(t *testing.T) {
	t.Parallel()
	m, _ := send(fourLevels(), "/")
	m, _, _ = m.Update(tea.PasteMsg{Content: " slow "})
	assert.Equal(t, "slow", m.Filter())
	// Not typing, a paste is ignored.
	m, _ = send(m, "enter")
	m, _, _ = m.Update(tea.PasteMsg{Content: "drain"})
	assert.Equal(t, "slow", m.Filter())
}

func TestUpdate_CloseAndQuit(t *testing.T) {
	t.Parallel()
	_, res := send(fourLevels(), "`")
	assert.Equal(t, Closed, res.Outcome)
	_, res = send(fourLevels(), "q")
	assert.Equal(t, Quit, res.Outcome)
}

// A filter that hides everything says so and how to undo it.
func TestRefresh_NoMatches(t *testing.T) {
	t.Parallel()
	m := fourLevels().SetFilter("zzz")
	shown, total := m.Counts()
	assert.Equal(t, 0, shown)
	assert.Equal(t, 4, total)
	assert.Contains(t, m.View(), "no entries match · esc clears the filters")
}

// Following, a refresh shows new entries at the bottom; paused, it keeps
// the offset. end resumes following.
func TestRefresh_FollowsOnlyAtBottom(t *testing.T) {
	t.Parallel()
	ring := many(40)
	m := open(ring)
	assert.True(t, m.Following())

	slog.New(ring).Info("newest")
	m = m.Refresh(80, 7, styles)
	assert.True(t, m.Following())
	assert.Contains(t, m.View(), "newest")

	m, _ = send(m, "home")
	assert.False(t, m.Following())
	assert.Contains(t, ansi.Strip(m.View()), "PAUSED")
	slog.New(ring).Info("after pause")
	m = m.Refresh(80, 7, styles)
	vp := m.Viewport()
	assert.Equal(t, 0, vp.YOffset())
	assert.NotContains(t, m.View(), "after pause")

	m, _ = send(m, "end")
	assert.True(t, m.Following())
	assert.Contains(t, m.View(), "after pause")
}

func TestUpdate_ScrollKeysPause(t *testing.T) {
	t.Parallel()
	m, _ := send(open(many(40)), "pgup")
	assert.False(t, m.Following())
}

// Refresh lays out at the size given: the panel's borders come off the
// width, and the frame is as tall as the body plus crumb, panel edges and
// footer.
func TestRefresh_Size(t *testing.T) {
	t.Parallel()
	m := open(many(40)).Refresh(100, 10, styles)
	vp := m.Viewport()
	assert.Equal(t, 98, vp.Width())
	assert.Equal(t, 10, vp.Height())
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	assert.Len(t, lines, 10+4)
	assert.Contains(t, lines[len(lines)-2], " 40 entries ─╯")
}

// The footer drops keys from the right to fit a narrow screen.
func TestView_FooterTrims(t *testing.T) {
	t.Parallel()
	m := open(many(3)).Refresh(20, 5, styles)
	assert.Equal(t, " / filter  l level", ansi.Strip(m.footer()))
	m = m.Refresh(1, 5, styles)
	assert.Empty(t, m.footer())
}

// A crumb too wide for the screen is truncated.
func TestView_CrumbTruncates(t *testing.T) {
	t.Parallel()
	m := open(many(3)).SetFilter(strings.Repeat("y", 100))
	crumb := m.crumb()
	assert.Equal(t, 80, lipgloss.Width(crumb))
	assert.Contains(t, crumb, "…")
}

// View renders without changing the Model.
func TestView_NoSideEffects(t *testing.T) {
	t.Parallel()
	ring := many(40)
	m := open(ring)
	m, _ = send(m, "/", "l", "i")
	slog.New(ring).Info("not yet refreshed")
	// %#v, not reflect.DeepEqual: the viewport holds a func, which
	// DeepEqual never finds equal.
	before := fmt.Sprintf("%#v", m)
	first := m.View()
	assert.Equal(t, first, m.View())
	assert.Equal(t, before, fmt.Sprintf("%#v", m))
	assert.NotContains(t, first, "not yet refreshed")
}

func TestTick(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, Tick())
}
