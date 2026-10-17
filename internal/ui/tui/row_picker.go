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
type rowPickerState struct {
	title      string
	verb       string // what enter does, for the footer (e.g. "go")
	targets    []domain.Scope
	cursor     int
	choose     func(domain.Scope) tea.Cmd
	returnView common.ViewMode
}

// openRowPicker shows the row picker over targets; choose runs on enter.
//
//nolint:unparam // returns nil so callers can `return m.openRowPicker(...)`
func (m *Model) openRowPicker(title, verb string, targets []domain.Scope, choose func(domain.Scope) tea.Cmd) tea.Cmd {
	m.rowPicker = rowPickerState{title: title, verb: verb, targets: targets, choose: choose, returnView: m.viewMode}
	m.viewMode = common.RowPickerView
	return nil
}

// updateRowPickerView moves the picker cursor (up/down, k/j), runs the
// choice on enter, and cancels on esc. Other keys are swallowed; ctrl+c
// quits.
func (m *Model) updateRowPickerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	g := &m.rowPicker
	switch {
	case key.Matches(km, keys.Quit):
		m.cancelInFlight()
		return m, tea.Quit
	case km.String() == "up" || km.String() == "k":
		if g.cursor > 0 {
			g.cursor--
		}
	case km.String() == "down" || km.String() == "j":
		if g.cursor < len(g.targets)-1 {
			g.cursor++
		}
	case km.String() == "enter":
		t, choose := g.targets[g.cursor], g.choose
		m.viewMode = g.returnView
		m.rowPicker = rowPickerState{}
		return m, choose(t)
	case km.String() == "esc":
		m.viewMode = g.returnView
		m.rowPicker = rowPickerState{}
	}
	return m, nil
}

// rowPickerView renders the picker: one line per target, category then name.
// Every line is padded to the same width so the centered block stays
// left-aligned (CenterText centers each line on its own).
func (m *Model) rowPickerView() string {
	g := m.rowPicker
	catWidth := 0
	for _, t := range g.targets {
		catWidth = max(catWidth, len(t.Category.String()))
	}
	lines := make([]string, 0, len(g.targets)+4)
	lines = append(lines, g.title, "")
	for i, t := range g.targets {
		marker := "  "
		if i == g.cursor {
			marker = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-*s  %s", marker, catWidth, t.Category, t.Name))
	}
	lines = append(lines, "", fmt.Sprintf("[↑/↓] select   [enter] %s   [esc] cancel", g.verb))
	width := 0
	for _, l := range lines {
		width = max(width, lipgloss.Width(l))
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", width-lipgloss.Width(l))
	}
	return strings.Join(lines, "\n")
}
