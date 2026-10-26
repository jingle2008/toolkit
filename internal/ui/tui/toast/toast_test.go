package toast

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testStyles = Styles{
	Info:   lipgloss.Color("#0000FF"),
	Warn:   lipgloss.Color("#FFFF00"),
	Error:  lipgloss.Color("#FF8080"),
	Hint:   lipgloss.Color("#808080"),
	LogKey: "`",
}

func TestShow_SetsStateAndReturnsCmd(t *testing.T) {
	t.Parallel()
	m, cmd := Model{}.Show("boom", Error)
	got := m.Active()
	require.NotNil(t, got)
	assert.Equal(t, "boom", got.Msg)
	assert.Equal(t, Error, got.Sev)
	assert.NotZero(t, got.ID)
	assert.Nil(t, cmd, "an error toast stays until dismissed: no auto-dismiss tick")
	assert.True(t, m.Visible())

	_, cmd = m.Show("fyi", Info)
	assert.NotNil(t, cmd, "an info toast's auto-dismiss tick")
}

func TestShow_MonotonicIDs(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("first", Error)
	first := m.Active().ID
	m, _ = m.Show("second", Info)
	assert.Greater(t, m.Active().ID, first)
}

func TestShowError_SetsNext(t *testing.T) {
	t.Parallel()
	m, cmd := Model{}.ShowError("save failed", "^s retry")
	assert.Nil(t, cmd)
	assert.Equal(t, "^s retry", m.Active().Next)
	assert.True(t, m.HasError())
}

func TestUpdate_ExpireClearsMatching(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("boom", Error)
	m, cmd := m.Update(ExpireMsg{ID: m.Active().ID})
	assert.Nil(t, cmd)
	assert.Nil(t, m.Active())
	assert.False(t, m.Visible())
}

func TestUpdate_IgnoresStaleIDAndOtherMsgs(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("first", Error)
	stale := m.Active().ID
	m, _ = m.Show("second", Error)
	m, _ = m.Update(ExpireMsg{ID: stale}) // removes "first" from older
	require.NotNil(t, m.Active())
	assert.Equal(t, "second", m.Active().Msg)
	assert.Empty(t, m.Older())

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, cmd)
	assert.Equal(t, "second", m.Active().Msg)
}

func TestShow_StacksAndPromotes(t *testing.T) {
	t.Parallel()
	var m Model
	for _, s := range []string{"one", "two", "three", "four"} {
		m, _ = m.Show(s, Info)
	}
	assert.Equal(t, "four", m.Active().Msg)
	older := m.Older()
	require.Len(t, older, MaxStacked-1, "at most three show")
	assert.Equal(t, "two", older[0].Msg)

	m, _ = m.Update(ExpireMsg{ID: older[0].ID})
	assert.Len(t, m.Older(), 1)
	m, _ = m.Update(ExpireMsg{ID: m.Active().ID})
	assert.Equal(t, "three", m.Active().Msg, "the next older takes its place")
	assert.Empty(t, m.Older())
}

// A full stack pushes out an info or warning toast before an error.
func TestShow_ErrorSurvivesOverflow(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("export failed", Error)
	for _, s := range []string{"one", "two", "three", "four"} {
		m, _ = m.Show(s, Info)
	}
	assert.True(t, m.HasError())
	assert.Len(t, m.Older(), MaxStacked-1)
	assert.Equal(t, "export failed", m.Older()[0].Msg)
}

// With only errors left, the oldest error goes.
func TestShow_AllErrorsDropsOldest(t *testing.T) {
	t.Parallel()
	var m Model
	for _, s := range []string{"e1", "e2", "e3", "e4"} {
		m, _ = m.Show(s, Error)
	}
	older := m.Older()
	require.Len(t, older, 2)
	assert.Equal(t, "e2", older[0].Msg)
}

// Value semantics: an earlier copy is unaffected by later changes.
func TestModel_CopiesAreIndependent(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("a", Error)
	m, _ = m.Show("b", Info)
	m, _ = m.Show("c", Info)
	before := m
	_ = m.DismissErrors()
	m2, _ := m.Show("d", Info)
	_ = m2
	assert.Equal(t, []string{"a", "b"}, msgs(before.Older()))
	assert.Equal(t, "c", before.Active().Msg)

	before.Active().Msg = "mutated"
	assert.Equal(t, "c", before.Active().Msg, "Active returns a copy")
}

func msgs(ts []Toast) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Msg)
	}
	return out
}

func TestDismissErrors(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("drain failed", Error)
	m, _ = m.Show("copied", Info)
	m, _ = m.Show("save failed", Error)
	m = m.DismissErrors()
	assert.False(t, m.HasError())
	require.NotNil(t, m.Active())
	assert.Equal(t, "copied", m.Active().Msg, "the info toast stays until it expires")
	assert.Empty(t, m.Older())

	m = m.DismissErrors()
	assert.Equal(t, "copied", m.Active().Msg)
	assert.False(t, Model{}.HasError())
}

func TestView_EmptyWhenNone(t *testing.T) {
	t.Parallel()
	assert.Empty(t, Model{}.View(40, testStyles))
	m, _ := Model{}.Show("x", Info)
	assert.Empty(t, m.viewActive(0, testStyles))
}

func TestView_ContainsMessageAndTitle(t *testing.T) {
	t.Parallel()
	for sev, title := range map[Severity]string{Info: "Info", Warn: "! Warning", Error: "✗ Error"} {
		m, _ := Model{}.Show("kubectl: connection refused", sev)
		got := ansi.Strip(m.View(60, testStyles))
		assert.Contains(t, got, "kubectl", sev)
		assert.Contains(t, got, title, sev)
	}
}

func TestView_ErrorUsesErrorColor(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("boom", Error)
	assert.Contains(t, m.View(40, testStyles), "38;2;255;128;128")
}

func TestView_InfoTruncatesToOneLine(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show(strings.Repeat("word ", 60), Info)
	lines := strings.Split(ansi.Strip(m.View(40, testStyles)), "\n")
	assert.Len(t, lines, 3, "borders and one message line")
	assert.Contains(t, lines[1], "…")
	for _, l := range lines {
		assert.LessOrEqual(t, lipgloss.Width(l), 40)
	}
}

func TestView_ErrorWrapsAndSaysHowToDismiss(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.ShowError(strings.Repeat("word ", 60), "r retry")
	box := ansi.Strip(m.View(60, testStyles))
	lines := strings.Split(box, "\n")
	assert.Len(t, lines, MaxErrorLines+3, "borders, %d message lines and the hint", MaxErrorLines)
	assert.Contains(t, box, "…", "an over-long error is cut on its last line")
	assert.Contains(t, box, "r retry · esc dismiss · ` log")
}

func TestView_StacksOldestOnTop(t *testing.T) {
	t.Parallel()
	m, _ := Model{}.Show("first", Info)
	m, _ = m.Show("second", Warn)
	got := ansi.Strip(m.View(40, testStyles))
	assert.Less(t, strings.Index(got, "first"), strings.Index(got, "second"))
}
