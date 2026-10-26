package rowpicker

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

var styles = Styles{Muted: lipgloss.Color("8"), Accent: lipgloss.Color("4")}

func targets(n int) []domain.Scope {
	out := make([]domain.Scope, n)
	for i := range out {
		out[i] = domain.Scope{Category: domain.DedicatedAICluster, Name: fmt.Sprintf("row-%02d", i+1)}
	}
	return out
}

// send feeds keys to m in a modal of 20 body rows and returns the last
// Result.
func send(m Model, ks ...string) (Model, Result) {
	var res Result
	for _, k := range ks {
		m, res = m.Update(press(k), 20)
	}
	return m, res
}

func TestNew(t *testing.T) {
	t.Parallel()
	m := New("Go to", "go", targets(3))
	assert.Equal(t, "Go to", m.Title())
	assert.Len(t, m.Targets(), 3)
	assert.Empty(t, m.Filter())
	assert.Zero(t, m.Cursor())
	assert.False(t, m.Filterable())
	assert.True(t, New("", "", targets(FilterMin+1)).Filterable())
}

func TestUpdate_ShortListMovesWithJK(t *testing.T) {
	t.Parallel()
	m, res := send(New("Pick", "go", targets(3)), "j", "j", "j")
	assert.Equal(t, None, res.Outcome)
	assert.Equal(t, 2, m.Cursor(), "clamped at the last row")
	assert.Empty(t, m.Filter(), "no filter to type into")
	m, _ = send(m, "k", "up", "ctrl+p")
	assert.Zero(t, m.Cursor(), "clamped at the first row")
	m, _ = send(m, "ctrl+n", "down")
	assert.Equal(t, 2, m.Cursor())
	m, _ = send(m, "x")
	assert.Equal(t, 2, m.Cursor(), "other keys are swallowed")
}

func TestUpdate_ChooseAndCancel(t *testing.T) {
	t.Parallel()
	m := New("Pick", "go", targets(3))
	_, res := send(m, "down", "enter")
	assert.Equal(t, Result{Outcome: Chosen, Scope: targets(3)[1]}, res)
	_, res = send(m, "esc")
	assert.Equal(t, Result{Outcome: Cancelled}, res)
	_, res = m.Update(tea.WindowSizeMsg{}, 20)
	assert.Equal(t, None, res.Outcome, "non-key messages are swallowed")
}

func TestUpdate_FilterPastFilterMin(t *testing.T) {
	t.Parallel()
	m, _ := send(New("Pick", "go", targets(12)), "1", "j")
	assert.Equal(t, "1j", m.Filter(), "j types into the filter")
	assert.Empty(t, m.Visible())
	assert.Contains(t, ansi.Strip(m.View(20, styles)), "no matches")
	m, res := send(m, "enter")
	assert.Equal(t, None, res.Outcome, "enter on no match is a no-op")

	m, _ = send(m, "backspace")
	assert.Equal(t, "1", m.Filter())
	require.Len(t, m.Visible(), 4) // row-01, row-10, row-11, row-12
	_, res = send(m, "down", "enter")
	assert.Equal(t, "row-10", res.Scope.Name)

	m, _ = send(m, "backspace", "backspace")
	assert.Empty(t, m.Filter(), "backspace on empty is a no-op")
	m, _ = send(m, "ctrl+n", "ctrl+n")
	m, _ = send(m, "r")
	assert.Zero(t, m.Cursor(), "editing the filter returns to the first match")
}

func TestUpdate_FilterMatchesCategory(t *testing.T) {
	t.Parallel()
	ts := append(targets(9), domain.Scope{Category: domain.Tenant, Name: "acme"})
	m, _ := send(New("Pick", "go", ts), "t", "e", "n")
	require.Len(t, m.Visible(), 1)
	assert.Equal(t, "acme", m.Visible()[0].Name)
}

func TestWithCursor_Clamps(t *testing.T) {
	t.Parallel()
	m := New("Pick", "go", targets(3))
	assert.Equal(t, 1, m.WithCursor(1).Cursor())
	assert.Equal(t, 2, m.WithCursor(9).Cursor())
	assert.Zero(t, m.WithCursor(-1).Cursor())
}

func TestView_Scrolls(t *testing.T) {
	t.Parallel()
	const maxBody = 8
	m := New("Pick", "go", targets(30))
	n := m.Rows(maxBody)
	assert.Equal(t, maxBody-3, n, "less the filter line, blank and indicator")
	assert.Equal(t, maxBody-1, New("", "", targets(3)).Rows(maxBody))
	assert.Equal(t, 1, m.Rows(0), "at least one row")
	for range n + 2 {
		m, _ = m.Update(press("down"), maxBody)
	}
	out := ansi.Strip(m.View(maxBody, styles))
	assert.Contains(t, out, fmt.Sprintf("row-%02d", n+3), "the cursor row is in view")
	assert.NotContains(t, out, "row-01 ", "the top scrolled away")
	assert.Contains(t, out, fmt.Sprintf("4–%d of 30", n+3))
	assert.Contains(t, out, "type to filter")
}

func TestView_KeepsSizeWhileFiltering(t *testing.T) {
	t.Parallel()
	m := New("Pick", "go", targets(12))
	before := m.View(20, styles)
	m, _ = send(m, "1", "1")
	after := m.View(20, styles)
	assert.Equal(t, lipgloss.Width(before), lipgloss.Width(after))
	assert.Equal(t, lipgloss.Height(before), lipgloss.Height(after))
}

func TestView_ShortList(t *testing.T) {
	t.Parallel()
	out := ansi.Strip(New("Go to", "go", targets(3)).WithCursor(1).View(20, styles))
	assert.Contains(t, out, "Go to")
	assert.Contains(t, out, "▸ ")
	assert.Contains(t, out, "enter go")
	assert.NotContains(t, out, "filter:")
	assert.NotContains(t, out, "type to filter")
	for _, line := range strings.Split(out, "\n") {
		assert.Equal(t, lipgloss.Width(strings.Split(out, "\n")[0]), lipgloss.Width(line), "every line is as wide")
	}
}
