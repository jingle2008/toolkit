// Package rowpicker is the TUI's modal row picker: a titled list of
// scopes (a category and a name) to choose one from. It backs go-to-related
// and the DAC choice for an imported model's metrics. Not to be confused
// with the CSV export's path picker.
//
// up/down (ctrl+p/ctrl+n) move the cursor, enter chooses, esc cancels.
// Past FilterMin rows, typed text and backspace edit a filter over the
// rows, matched case-insensitively against category and name; without a
// filter, k/j move the cursor too. The box keeps the size of the
// unfiltered list while a filter narrows it, so it neither shrinks nor
// moves as it is typed.
//
// Model is a value type in the Bubble Tea nested-model style: Update
// returns the updated Model, which the parent stores back, and a Result
// that says whether a row was chosen or the picker cancelled. The Result
// is returned synchronously, so the parent closes the picker and acts on
// the choice within the same keypress. The Model holds no closures: what
// choosing does is the parent's, keyed by Result.Scope.
package rowpicker

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// FilterMin is the row count above which the picker shows a filter.
const FilterMin = 8

// Outcome is what a keypress decided.
type Outcome int

// The outcomes: None leaves the picker open.
const (
	None Outcome = iota
	Chosen
	Cancelled
)

// Result is what Update decided: on Chosen the parent acts on Scope;
// either Chosen or Cancelled closes the picker.
type Result struct {
	Outcome Outcome
	Scope   domain.Scope
}

// Model is an open picker; the zero Model is none. cursor and offset
// index the filtered rows.
type Model struct {
	title   string
	verb    string // what enter does, for the hints (e.g. "go")
	targets []domain.Scope
	filter  string
	cursor  int
	offset  int // first visible row when the list scrolls
}

// New opens a picker titled title over targets; verb names what enter
// does in the hints.
func New(title, verb string, targets []domain.Scope) Model {
	return Model{title: title, verb: verb, targets: targets}
}

// Title is the picker's title.
func (m Model) Title() string { return m.title }

// Targets are every row, filtered or not.
func (m Model) Targets() []domain.Scope { return m.targets }

// Filter is the filter as typed.
func (m Model) Filter() string { return m.filter }

// Cursor indexes the highlighted row of Visible.
func (m Model) Cursor() int { return m.cursor }

// WithCursor returns m with the cursor on row i of Visible, clamped.
func (m Model) WithCursor(i int) Model {
	m.cursor = min(max(i, 0), max(len(m.Visible())-1, 0))
	return m
}

// Filterable reports whether the picker has enough rows to filter.
func (m Model) Filterable() bool { return len(m.targets) > FilterMin }

// Visible returns the targets matching the filter, case-insensitively, in
// category or name.
func (m Model) Visible() []domain.Scope {
	if m.filter == "" {
		return m.targets
	}
	f := strings.ToLower(m.filter)
	var out []domain.Scope
	for _, t := range m.targets {
		if strings.Contains(strings.ToLower(t.Category.String()+" "+t.Name), f) {
			out = append(out, t)
		}
	}
	return out
}

// Rows is how many target rows the picker shows before scrolling, given
// maxBody body rows: less the filter line and scroll indicator.
func (m Model) Rows(maxBody int) int {
	reserved := 1 // scroll indicator
	if m.Filterable() {
		reserved += 2 // filter line and the blank under it
	}
	return max(maxBody-reserved, 1)
}

// Update resolves a message while the picker is open, in a modal of
// maxBody body rows: a cursor move, a filter edit, enter (choosing the
// highlighted row; a no-op with no rows) or esc. Other messages and keys
// are swallowed. Quitting the program is the parent's: it intercepts its
// quit key before Update.
func (m Model) Update(msg tea.Msg, maxBody int) (Model, Result) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, Result{}
	}
	k := km.String()
	switch {
	case m.moveCursor(k):
	case k == "enter":
		rows := m.Visible()
		if len(rows) == 0 {
			return m, Result{}
		}
		return m, Result{Outcome: Chosen, Scope: rows[m.cursor]}
	case k == "esc":
		return m, Result{Outcome: Cancelled}
	case m.Filterable():
		m.editFilter(km)
	}
	m.offset = view.ScrollWindow(m.cursor, m.offset, m.Rows(maxBody))
	return m, Result{}
}

// moveCursor moves the cursor for up/down (ctrl+p/ctrl+n, and k/j when
// there is no filter to type into) and reports whether k was one of them.
func (m *Model) moveCursor(k string) bool {
	switch {
	case k == "up" || k == "ctrl+p" || (k == "k" && !m.Filterable()):
		m.cursor = max(m.cursor-1, 0)
	case k == "down" || k == "ctrl+n" || (k == "j" && !m.Filterable()):
		m.cursor = min(m.cursor+1, max(len(m.Visible())-1, 0))
	default:
		return false
	}
	return true
}

// editFilter applies a typed character or backspace to the filter and
// puts the cursor back on the first match.
func (m *Model) editFilter(km tea.KeyPressMsg) {
	switch {
	case km.String() == "backspace":
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
		}
	case km.Text != "":
		m.filter += km.Text
	default:
		return
	}
	m.cursor, m.offset = 0, 0
}

// Styles are the colors the picker is drawn in, taken from the parent's
// theme.
type Styles struct {
	Muted  color.Color // category badges, filter label, indicator, hints
	Accent color.Color // the border, title and highlighted row
}

// View draws the picker in a modal of maxBody body rows: an optional
// filter line, one row per visible target (a left-aligned category badge,
// then the name), and a scroll indicator when the rows don't all fit.
func (m Model) View(maxBody int, st Styles) string {
	rows := m.Visible()
	n := m.Rows(maxBody)
	start := min(m.offset, max(len(rows)-n, 0))
	end := min(start+n, len(rows))

	catWidth := 0
	for _, t := range m.targets {
		catWidth = max(catWidth, lipgloss.Width(t.Category.String())+2)
	}
	badge := lipgloss.NewStyle().Foreground(st.Muted).Width(catWidth).Padding(0, 1)
	cursorStyle := lipgloss.NewStyle().Foreground(st.Accent).Bold(true)

	row := func(marker, name string, t domain.Scope) string {
		return marker + badge.Render(t.Category.String()) + "  " + name
	}
	var lines []string
	if m.Filterable() {
		lines = append(lines, lipgloss.NewStyle().Foreground(st.Muted).Render("filter: ")+m.filter+"█", "")
	}
	for i := start; i < end; i++ {
		t := rows[i]
		marker, name := "  ", t.Name
		if i == m.cursor {
			marker, name = cursorStyle.Render("▸ "), cursorStyle.Render(t.Name)
		}
		lines = append(lines, row(marker, name, t))
	}
	if len(rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(st.Muted).Render("  no matches"))
	}
	if len(rows) > n {
		lines = append(lines, lipgloss.NewStyle().Foreground(st.Muted).
			Render(fmt.Sprintf("  %d–%d of %d", start+1, end, len(rows))))
	}
	hints := fmt.Sprintf("↑/↓ select · enter %s · esc cancel", m.verb)
	if m.Filterable() {
		hints = fmt.Sprintf("type to filter · ↑/↓ select · enter %s · esc cancel", m.verb)
	}
	// As wide as every target's row and as tall as the unfiltered list, so
	// filtering neither narrows nor shortens the box.
	bodyW, bodyH := 0, min(n, len(m.targets))
	for _, t := range m.targets {
		bodyW = max(bodyW, lipgloss.Width(row("  ", t.Name, t)))
	}
	if m.Filterable() {
		bodyH += 2
	}
	if len(m.targets) > n {
		bodyH++
	}
	return view.FixedModal(m.title, strings.Join(lines, "\n"), hints, st.Accent, st.Muted, 0, bodyW, bodyH)
}
