package palette

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

var testStyles = Styles{Accent: lipgloss.Color("#ff8800"), Muted: color.Gray{Y: 128}}

func press(k string) tea.KeyPressMsg {
	switch k {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+p", "ctrl+n":
		return tea.KeyPressMsg{Code: rune(k[5]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

func sample() []Item {
	return []Item{
		{Kind: KindCategory, Label: "DedicatedAICluster", Hint: "dac"},
		{Kind: KindCategory, Label: "GPUNode", Hint: "gpun"},
		{Kind: "env", Label: "dp-prod-yny", Hint: "oc1 ap-chuncheon-1"},
		{Kind: "action", Label: "Sort by Tenant", Hint: "s t"},
		{Kind: "toggle", Label: "Preview pane", Hint: "p"},
	}
}

func typeQuery(t *testing.T, m Model, q string) Model {
	t.Helper()
	for _, r := range q {
		var res Result
		m, res, _ = m.Update(press(string(r)), MaxRows)
		require.Equal(t, None, res.Outcome)
	}
	return m
}

func labels(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func TestNew_ListsEverythingRecentFirst(t *testing.T) {
	t.Parallel()
	items := sample()
	m := New(items, []string{items[3].ID(), "gone\x00missing", items[1].ID()})
	assert.Equal(t, items, m.Items())
	assert.Equal(t, 2, m.Recent(), "unknown recent IDs are skipped")
	assert.Equal(t, []string{"Sort by Tenant", "GPUNode", "DedicatedAICluster", "dp-prod-yny", "Preview pane"},
		labels(m.Matches()), "recent first, then the rest without repeats")
	assert.Empty(t, m.Query())
}

// A match never strings letters across label and hint: "dac" must not
// match an environment as d(p-prod) a(p-…) c(huncheon).
func TestSetQuery_NoCrossFieldMatches(t *testing.T) {
	t.Parallel()
	m := New([]Item{
		{Kind: "env", Label: "dp-prod-yny", Hint: "oc1 ap-chuncheon-1"},
		{Kind: KindCategory, Label: "DedicatedAICluster", Hint: "dac"},
	}, nil).SetQuery("dac")
	require.Len(t, m.Matches(), 1)
	assert.Equal(t, "DedicatedAICluster", m.Matches()[0].Label)
	assert.Zero(t, m.Recent(), "no recent group under a query")
}

func TestSetQuery_MatchesHints(t *testing.T) {
	t.Parallel()
	m := New(sample(), nil).SetQuery("chun")
	got := labels(m.Matches())
	require.NotEmpty(t, got)
	assert.Equal(t, []string{"dp-prod-yny"}, got, "found through its hint only")
}

func TestUpdate_ExactAliasChoosesCategory(t *testing.T) {
	t.Parallel()
	m := typeQuery(t, New(sample(), nil), "gpun")
	assert.Equal(t, "gpun", m.Query())
	_, res, _ := m.Update(press("enter"), MaxRows)
	assert.Equal(t, Chosen, res.Outcome)
	assert.Equal(t, -1, res.Index)
	assert.Equal(t, domain.GPUNode, res.Category)
	assert.Equal(t, Item{Kind: KindCategory, Label: "GPUNode"}, res.Item)
}

func TestUpdate_ChoosesHighlightedMatch(t *testing.T) {
	t.Parallel()
	m := typeQuery(t, New(sample(), nil), "preview")
	_, res, _ := m.Update(press("enter"), MaxRows)
	assert.Equal(t, Chosen, res.Outcome)
	assert.Equal(t, 4, res.Index, "indexes the items the palette was opened with")
	assert.Equal(t, "Preview pane", res.Item.Label)
}

func TestUpdate_NoMatchesStaysOpen(t *testing.T) {
	t.Parallel()
	m := typeQuery(t, New(sample(), nil), "zzzqqq")
	assert.Empty(t, m.Matches())
	assert.Contains(t, ansi.Strip(m.View(80, MaxRows, testStyles)), "no matches")
	_, res, _ := m.Update(press("enter"), MaxRows)
	assert.Equal(t, None, res.Outcome)
}

func TestUpdate_EscCloses(t *testing.T) {
	t.Parallel()
	_, res, _ := New(sample(), nil).Update(press("esc"), MaxRows)
	assert.Equal(t, Closed, res.Outcome)
}

func TestUpdate_IgnoresOtherMessages(t *testing.T) {
	t.Parallel()
	m := New(sample(), nil)
	got, res, cmd := m.Update(tea.WindowSizeMsg{Width: 10}, MaxRows)
	assert.Equal(t, None, res.Outcome)
	assert.Nil(t, cmd)
	assert.Equal(t, m.Matches(), got.Matches())
}

func TestUpdate_PasteFeedsQuery(t *testing.T) {
	t.Parallel()
	m, res, _ := New(sample(), nil).Update(tea.PasteMsg{Content: "dac"}, MaxRows)
	assert.Equal(t, None, res.Outcome)
	assert.Equal(t, "dac", m.Query())
	assert.Equal(t, "DedicatedAICluster", m.Matches()[0].Label)
}

func TestUpdate_CursorMovesAndScrolls(t *testing.T) {
	t.Parallel()
	m := New(sample(), nil)
	const rows = 2
	for range 10 {
		m, _, _ = m.Update(press("down"), rows)
	}
	assert.Equal(t, len(sample())-1, m.Cursor(), "stops at the last match")
	view := ansi.Strip(m.View(80, rows, testStyles))
	assert.Contains(t, view, "▸ Preview pane")
	assert.Contains(t, view, "4–5 of 5")

	m, _, _ = m.Update(press("ctrl+p"), rows)
	assert.Equal(t, 3, m.Cursor())
	m, _, _ = m.Update(press("ctrl+n"), rows)
	assert.Equal(t, 4, m.Cursor())
	for range 10 {
		m, _, _ = m.Update(press("up"), rows)
	}
	assert.Zero(t, m.Cursor(), "stops at the first match")
	assert.Contains(t, ansi.Strip(m.View(80, rows, testStyles)), "1–2 of 5")
}

// The box keeps the unfiltered list's size while a query narrows it.
func TestView_HoldsSizeWhileFiltering(t *testing.T) {
	t.Parallel()
	m := New(sample(), nil)
	before := m.View(200, MaxRows, testStyles)
	after := typeQuery(t, m, "gpunode").View(200, MaxRows, testStyles)
	assert.Equal(t, lipgloss.Height(before), lipgloss.Height(after))
	assert.Equal(t, lipgloss.Width(before), lipgloss.Width(after))
	assert.Equal(t, MaxWidth, lipgloss.Width(before), "capped at MaxWidth")
	assert.Contains(t, after, "\x1b[1;", "matched characters are highlighted")
}

func TestView_GroupsNamedOnce(t *testing.T) {
	t.Parallel()
	items := sample()
	m := New(items, []string{items[4].ID()})
	view := ansi.Strip(m.View(80, MaxRows, testStyles))
	assert.Equal(t, 1, strings.Count(view, "recent"))
	assert.Equal(t, 1, strings.Count(view, " category"))
	assert.Contains(t, view, "Command palette")
}

func TestRemember_CapsAndDedupes(t *testing.T) {
	t.Parallel()
	var recent []string
	for i := range RecentMax + 2 {
		recent = Remember(recent, Item{Kind: "action", Label: fmt.Sprint(i)})
	}
	prev := recent
	recent = Remember(recent, Item{Kind: "action", Label: "3"})
	assert.Len(t, recent, RecentMax)
	assert.Equal(t, Item{Kind: "action", Label: "3"}.ID(), recent[0])
	assert.Equal(t, 1, strings.Count(strings.Join(recent, "|"), "\x003"))
	assert.Equal(t, Item{Kind: "action", Label: "6"}.ID(), prev[0], "the old slice is left alone")
}

func TestHighlight(t *testing.T) {
	t.Parallel()
	hit := lipgloss.NewStyle().Bold(true)
	assert.Equal(t, "abc", highlight("abc", nil, hit))
	got := highlight("abc", []int{1}, hit)
	assert.Equal(t, "abc", ansi.Strip(got))
	assert.Contains(t, got, hit.Render("b"))
}
