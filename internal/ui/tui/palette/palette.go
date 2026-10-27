// Package palette is the TUI's command palette.
//
// The palette lists Items — categories to open, environments to switch
// to, actions on the selected row, toggles — under a query line. Typing
// ranks them by fuzzy match (sahilm/fuzzy) with the matched characters
// highlighted; with no query the recently run items come first. ↑/↓
// (ctrl+p/ctrl+n) move the cursor, enter chooses, esc closes. An exact
// category alias wins on enter whatever the ranking, so :dac⏎ opens
// DedicatedAICluster.
//
// Model is a value type in the Bubble Tea nested-model style: Update
// returns the updated Model, which the parent stores back, and a Result
// that says what was chosen. The Result is returned synchronously, not
// as a message, so the parent closes the palette and runs the choice
// within the same keypress. Items hold no closures: what an item does is
// the parent's, keyed by Result.Index.
package palette

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/jingle2008/toolkit/internal/domain"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// MaxRows caps how many items the palette lists before scrolling.
const MaxRows = 12

// MaxWidth caps the palette's outer width.
const MaxWidth = 72

// RecentMax is how many recently run items the palette lists first.
const RecentMax = 5

// KindCategory is the Kind of an item that opens a category; an exact
// alias in the query chooses one.
const KindCategory = "category"

// Item is one thing the palette offers.
type Item struct {
	Kind  string // "category", "env", "action" or "toggle", shown on the right
	Label string
	Hint  string // aliases, or the item's key, shown muted beside the kind
}

// ID names the item across palette openings, for the recent list.
func (it Item) ID() string { return it.Kind + "\x00" + it.Label }

// Remember puts it first in recent, the IDs of the items run, newest
// first, keeping RecentMax at most and no repeats.
func Remember(recent []string, it Item) []string {
	id := it.ID()
	recent = slices.DeleteFunc(slices.Clone(recent), func(r string) bool { return r == id })
	recent = append([]string{id}, recent...)
	if len(recent) > RecentMax {
		recent = recent[:RecentMax]
	}
	return recent
}

// Outcome is what a keypress decided.
type Outcome int

// The outcomes: None leaves the palette open.
const (
	None Outcome = iota
	Chosen
	Closed
)

// Result is what Update decided. On Chosen the parent runs the item at
// Index among the Items the palette was opened with — or, when Index is
// -1, opens Category, which the query named by an exact alias. Item is
// the choice, for the recent list. On Closed it just closes.
type Result struct {
	Outcome  Outcome
	Item     Item
	Index    int
	Category domain.Category
}

// Model is the open palette; the zero Model is none. matches holds the
// items the query keeps, best first; cursor and offset index it. With no
// query the first recent matches are the recently run items.
type Model struct {
	input     textinput.Model
	items     []Item
	recentIDs []string
	matches   []fuzzy.Match
	recent    int
	cursor    int
	offset    int
}

// New opens the palette on items with every one listed, the ones recent
// names first, ready for a query.
func New(items []Item, recent []string) Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Focus()
	m := Model{input: ti, items: items, recentIDs: recent}
	m.refilter()
	return m
}

// Items returns what the palette offers, in the order it was opened with.
func (m Model) Items() []Item { return m.items }

// Matches returns the items the query keeps, best first.
func (m Model) Matches() []Item {
	out := make([]Item, len(m.matches))
	for i, mt := range m.matches {
		out[i] = m.items[mt.Index]
	}
	return out
}

// Recent is how many of the first Matches are recently run items.
func (m Model) Recent() int { return m.recent }

// Cursor is the highlighted match's index in Matches.
func (m Model) Cursor() int { return m.cursor }

// Query is the query as typed.
func (m Model) Query() string { return m.input.Value() }

// SetQuery replaces the query and re-ranks the matches.
func (m Model) SetQuery(q string) Model {
	m.input.SetValue(q)
	m.refilter()
	return m
}

// refilter re-ranks the items for the current query: every item in order
// when it is empty. Otherwise the category an exact alias names comes
// first (it is what enter runs), then fuzzy matches on labels, then items
// found only through their hint (a category by one of its aliases).
// Labels and hints are matched separately so a match never strings
// letters across both.
func (m *Model) refilter() {
	m.cursor, m.offset = 0, 0
	q := strings.TrimSpace(m.input.Value())
	m.matches = make([]fuzzy.Match, 0, len(m.items))
	m.recent = 0
	seen := make(map[int]bool, len(m.items))
	add := func(mt fuzzy.Match) {
		if !seen[mt.Index] {
			seen[mt.Index] = true
			m.matches = append(m.matches, mt)
		}
	}
	if q == "" {
		for _, id := range m.recentIDs {
			if i := slices.IndexFunc(m.items, func(it Item) bool { return it.ID() == id }); i >= 0 {
				add(fuzzy.Match{Index: i})
			}
		}
		m.recent = len(m.matches)
		for i := range m.items {
			add(fuzzy.Match{Index: i})
		}
		return
	}
	if cat, err := domain.ParseCategory(q); err == nil {
		for i, it := range m.items {
			if it.Kind == KindCategory && it.Label == cat.String() {
				add(fuzzy.Match{Index: i})
			}
		}
	}
	labels := make([]string, len(m.items))
	hints := make([]string, len(m.items))
	for i, it := range m.items {
		labels[i], hints[i] = it.Label, it.Hint
	}
	for _, mt := range fuzzy.Find(q, labels) {
		add(mt)
	}
	for _, mt := range fuzzy.Find(q, hints) {
		mt.MatchedIndexes = nil // they index the hint, not the label
		add(mt)
	}
}

// Update moves through the matches (↑/↓, ctrl+p/ctrl+n) keeping the
// cursor within a window of rows, chooses on enter, closes on esc, and
// feeds everything else, pastes included, to the query. Quitting the
// program is the parent's: it intercepts its quit key before Update.
func (m Model) Update(msg tea.Msg, rows int) (Model, Result, tea.Cmd) {
	if _, ok := msg.(tea.PasteMsg); ok {
		cmd := m.updateQuery(msg)
		return m, Result{}, cmd
	}
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, Result{}, nil
	}
	switch km.String() {
	case "esc":
		return m, Result{Outcome: Closed}, nil
	case "up", "ctrl+p":
		m.cursor = max(m.cursor-1, 0)
	case "down", "ctrl+n":
		m.cursor = min(m.cursor+1, max(len(m.matches)-1, 0))
	case "enter":
		return m, m.choose(), nil
	default:
		cmd := m.updateQuery(msg)
		return m, Result{}, cmd
	}
	m.offset = view.ScrollWindow(m.cursor, m.offset, rows)
	return m, Result{}, nil
}

// updateQuery feeds msg (a key or a paste) to the query and re-ranks the
// matches when it changed.
func (m *Model) updateQuery(msg tea.Msg) tea.Cmd {
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.refilter()
	}
	return cmd
}

// choose is enter: the exact alias, else the highlighted match. With no
// match it chooses nothing and the palette stays open, so the query can
// be edited.
func (m Model) choose() Result {
	if cat, err := domain.ParseCategory(m.input.Value()); err == nil {
		return Result{Outcome: Chosen, Item: Item{Kind: KindCategory, Label: cat.String()}, Index: -1, Category: cat}
	}
	if len(m.matches) == 0 {
		return Result{}
	}
	i := m.matches[m.cursor].Index
	return Result{Outcome: Chosen, Item: m.items[i], Index: i}
}

// Styles are the colors the palette is drawn in, taken from the parent's
// theme.
type Styles struct {
	Accent color.Color // border, cursor marker, matched characters
	Muted  color.Color // hints, kinds, key hints
}

// View draws the palette at most maxWidth columns wide (and at most
// MaxWidth), listing rows matches at a time: the query, then one row per
// visible match with its matched characters highlighted, its hint and
// its group. A group (recent, or the item's kind) is named on its first
// row only. The box is as tall as the unfiltered list, so it keeps its
// size while a query narrows it.
func (m Model) View(maxWidth, rows int, st Styles) string {
	width := min(maxWidth, MaxWidth)
	inner := width - 4
	n := rows
	start := min(m.offset, max(len(m.matches)-n, 0))
	end := min(start+n, len(m.matches))

	muted := lipgloss.NewStyle().Foreground(st.Muted)
	hit := lipgloss.NewStyle().Foreground(st.Accent).Bold(true)
	group := func(i int) string {
		if i < m.recent {
			return "recent"
		}
		return m.items[m.matches[i].Index].Kind
	}
	kindW := 0
	for i := range m.matches {
		kindW = max(kindW, len(group(i)))
	}
	lines := []string{m.input.View(), ""}
	for i := start; i < end; i++ {
		mt := m.matches[i]
		it := m.items[mt.Index]
		marker := "  "
		if i == m.cursor {
			marker = hit.Render("▸ ")
		}
		label := highlight(it.Label, mt.MatchedIndexes, hit)
		kind := ""
		if i == start || group(i) != group(i-1) {
			kind = group(i)
		}
		right := muted.Render(view.Truncate(it.Hint, inner/3) + "  " + fmt.Sprintf("%-*s", kindW, kind))
		gap := max(inner-2-lipgloss.Width(label)-lipgloss.Width(right), 1)
		lines = append(lines, marker+label+strings.Repeat(" ", gap)+right)
	}
	if len(m.matches) == 0 {
		lines = append(lines, muted.Render("  no matches"))
	}
	if len(m.matches) > n {
		lines = append(lines, muted.Render(fmt.Sprintf("  %d–%d of %d", start+1, end, len(m.matches))))
	}
	// As tall as the unfiltered list: the query, a blank, the rows, and the
	// count line when they scroll.
	bodyH := 2 + min(n, len(m.items))
	if len(m.items) > n {
		bodyH++
	}
	return view.FixedModal("Command palette", strings.Join(lines, "\n"),
		"↑/↓ select · enter run · esc close", st.Accent, st.Muted, width, 0, bodyH)
}

// highlight renders s with the runes at byte offsets idx in style hit.
func highlight(s string, idx []int, hit lipgloss.Style) string {
	if len(idx) == 0 {
		return s
	}
	marked := make(map[int]bool, len(idx))
	for _, i := range idx {
		marked[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if marked[i] {
			b.WriteString(hit.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
