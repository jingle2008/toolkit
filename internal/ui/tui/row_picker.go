package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// rowPickerState holds the modal row picker: a title, the rows to choose
// from, the highlighted one, what choosing does, and the view to restore.
// It backs go-to-related (o) and the DAC choice for an imported model's
// metrics (m). Not to be confused with the CSV export's path picker.
// Past pickerFilterMin rows, typing filters them; cursor and offset index
// the filtered rows.
type rowPickerState struct {
	title      string
	verb       string // what enter does, for the footer (e.g. "go")
	targets    []domain.Scope
	filter     string
	cursor     int
	offset     int // first visible row when the list scrolls
	choose     func(domain.Scope) tea.Cmd
	returnView common.ViewMode
}

// pickerFilterMin is the row count above which the picker shows a filter.
const pickerFilterMin = 8

// filterable reports whether the picker has enough rows to filter.
func (g *rowPickerState) filterable() bool { return len(g.targets) > pickerFilterMin }

// visible returns the targets matching the filter, case-insensitively, in
// category or name.
func (g *rowPickerState) visible() []domain.Scope {
	if g.filter == "" {
		return g.targets
	}
	f := strings.ToLower(g.filter)
	var out []domain.Scope
	for _, t := range g.targets {
		if strings.Contains(strings.ToLower(t.Category.String()+" "+t.Name), f) {
			out = append(out, t)
		}
	}
	return out
}

// openRowPicker shows the row picker over targets; choose runs on enter.
//
//nolint:unparam // returns nil so callers can `return m.openRowPicker(...)`
func (m *Model) openRowPicker(title, verb string, targets []domain.Scope, choose func(domain.Scope) tea.Cmd) tea.Cmd {
	m.rowPicker = rowPickerState{title: title, verb: verb, targets: targets, choose: choose, returnView: m.viewMode}
	m.viewMode = common.RowPickerView
	return nil
}

// updateRowPickerView moves the picker cursor (up/down, plus k/j when the
// picker has no filter), runs the choice on enter, and cancels on esc.
// With a filter, typed text and backspace edit it; other keys are
// swallowed. ctrl+c quits.
func (m *Model) updateRowPickerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	g := &m.rowPicker
	k := km.String()
	switch {
	case key.Matches(km, keys.Quit):
		return m, m.quit()
	case g.moveCursor(k):
	case k == "enter":
		rows := g.visible()
		if len(rows) == 0 {
			return m, nil
		}
		t, choose := rows[g.cursor], g.choose
		m.viewMode = g.returnView
		m.rowPicker = rowPickerState{}
		return m, choose(t)
	case k == "esc":
		m.viewMode = g.returnView
		m.rowPicker = rowPickerState{}
		return m, nil
	case g.filterable():
		m.editPickerFilter(km)
	}
	g.offset = scrollWindow(g.cursor, g.offset, m.pickerRows())
	return m, nil
}

// moveCursor moves the cursor for up/down (ctrl+p/ctrl+n, and k/j when
// there is no filter to type into) and reports whether k was one of them.
func (g *rowPickerState) moveCursor(k string) bool {
	switch {
	case k == "up" || k == "ctrl+p" || (k == "k" && !g.filterable()):
		g.cursor = max(g.cursor-1, 0)
	case k == "down" || k == "ctrl+n" || (k == "j" && !g.filterable()):
		g.cursor = min(g.cursor+1, max(len(g.visible())-1, 0))
	default:
		return false
	}
	return true
}

// editPickerFilter applies a typed character or backspace to the filter
// and puts the cursor back on the first match.
func (m *Model) editPickerFilter(km tea.KeyPressMsg) {
	g := &m.rowPicker
	switch {
	case km.String() == "backspace":
		if r := []rune(g.filter); len(r) > 0 {
			g.filter = string(r[:len(r)-1])
		}
	case km.Text != "":
		g.filter += km.Text
	default:
		return
	}
	g.cursor, g.offset = 0, 0
}

// pickerRows is how many target rows the picker shows before scrolling:
// the modal body less the filter line and scroll indicator.
func (m *Model) pickerRows() int {
	reserved := 1 // scroll indicator
	if m.rowPicker.filterable() {
		reserved += 2 // filter line and the blank under it
	}
	return max(m.modalMaxBody()-reserved, 1)
}

// scrollWindow returns the first visible row of a window of n rows that
// keeps cursor in view, moving offset as little as possible.
func scrollWindow(cursor, offset, n int) int {
	switch {
	case cursor < offset:
		return cursor
	case cursor >= offset+n:
		return cursor - n + 1
	}
	return offset
}

// rowPickerView renders the picker: an optional filter line, one row per
// visible target (a left-aligned category badge, then the name), and a scroll
// indicator when the rows don't all fit.
func (m *Model) rowPickerView() string {
	g := m.rowPicker
	p := m.theme.Palette
	rows := g.visible()
	n := m.pickerRows()
	start := min(g.offset, max(len(rows)-n, 0))
	end := min(start+n, len(rows))

	catWidth := 0
	for _, t := range g.targets {
		catWidth = max(catWidth, lipgloss.Width(t.Category.String())+2)
	}
	badge := lipgloss.NewStyle().Foreground(p.Muted).Width(catWidth).Padding(0, 1)
	cursorStyle := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)

	row := func(marker, name string, t domain.Scope) string {
		return marker + badge.Render(t.Category.String()) + "  " + name
	}
	var lines []string
	if g.filterable() {
		lines = append(lines, lipgloss.NewStyle().Foreground(p.Muted).Render("filter: ")+g.filter+"█", "")
	}
	for i := start; i < end; i++ {
		t := rows[i]
		marker, name := "  ", t.Name
		if i == g.cursor {
			marker, name = cursorStyle.Render("▸ "), cursorStyle.Render(t.Name)
		}
		lines = append(lines, row(marker, name, t))
	}
	if len(rows) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(p.Muted).Render("  no matches"))
	}
	if len(rows) > n {
		lines = append(lines, lipgloss.NewStyle().Foreground(p.Muted).
			Render(fmt.Sprintf("  %d–%d of %d", start+1, end, len(rows))))
	}
	hints := fmt.Sprintf("↑/↓ select · enter %s · esc cancel", g.verb)
	if g.filterable() {
		hints = fmt.Sprintf("type to filter · ↑/↓ select · enter %s · esc cancel", g.verb)
	}
	// As wide as every target's row and as tall as the unfiltered list, so
	// filtering neither narrows nor shortens the box.
	bodyW, bodyH := 0, min(n, len(g.targets))
	for _, t := range g.targets {
		bodyW = max(bodyW, lipgloss.Width(row("  ", t.Name, t)))
	}
	if g.filterable() {
		bodyH += 2
	}
	if len(g.targets) > n {
		bodyH++
	}
	return m.fixedModal(g.title, strings.Join(lines, "\n"), hints, p.Accent, 0, bodyW, bodyH)
}
