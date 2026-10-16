package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// goToOverlay holds the go-to-related picker: the rows the selected row
// relates to, the highlighted one, and the view to restore on cancel.
type goToOverlay struct {
	targets    []domain.Scope
	cursor     int
	returnView common.ViewMode
}

// goToTargets returns the rows the selected row relates to, through every
// relation into its category (the inverse of each drill-in). A target
// whose category is loaded but has no row of that name is dropped — e.g.
// a shared-catalog endpoint's model offered as an ImportedModel — so the
// picker never offers a dead jump. A target in a category not loaded yet
// is kept: jumping there loads it.
func (m *Model) goToTargets() []domain.Scope {
	src, ok := rowSources[m.category]
	if !ok || src.related == nil || m.dataset == nil {
		return nil
	}
	key := itemKeyFrom(m.category, m.selectedRawRow())
	if key == nil {
		return nil
	}
	var out []domain.Scope
	for _, t := range src.related(m.dataset, key) {
		if rowKnownMissing(m, t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// rowKnownMissing reports whether t's category has rows loaded and none
// is named t.Name.
func rowKnownMissing(m *Model, t domain.Scope) bool {
	rows, _ := computeTableRows(m.dataset, t.Category, nil, "", "", true, false)
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if len(r) > 0 && r[0] == t.Name {
			return false
		}
	}
	return true
}

// openGoTo handles g in list view: no targets is a toast, one target
// jumps straight there, several open the picker.
func (m *Model) openGoTo() tea.Cmd {
	targets := m.goToTargets()
	switch len(targets) {
	case 0:
		return m.showToast("no related rows to go to", toastInfo)
	case 1:
		return m.goTo(targets[0])
	default:
		m.goToPicker = goToOverlay{targets: targets, returnView: m.viewMode}
		m.viewMode = common.GoToView
		return nil
	}
}

// goTo navigates to t's category with t as the scope, so the list lands
// on t's row — the same mechanism as the parent jump.
func (m *Model) goTo(t domain.Scope) tea.Cmd {
	m.scope = &t
	return tea.Sequence(m.updateCategory(t.Category)...)
}

// updateGoToView moves the picker cursor (up/down, k/j), jumps on enter,
// and cancels on esc. Other keys are swallowed; ctrl+c quits.
func (m *Model) updateGoToView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	g := &m.goToPicker
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
		t := g.targets[g.cursor]
		m.viewMode = g.returnView
		m.goToPicker = goToOverlay{}
		return m, m.goTo(t)
	case km.String() == "esc":
		m.viewMode = g.returnView
		m.goToPicker = goToOverlay{}
	}
	return m, nil
}

// goToView renders the picker: one line per target, category then name.
// Every line is padded to the same width so the centered block stays
// left-aligned (CenterText centers each line on its own).
func (m *Model) goToView() string {
	g := m.goToPicker
	catWidth := 0
	for _, t := range g.targets {
		catWidth = max(catWidth, len(t.Category.String()))
	}
	lines := make([]string, 0, len(g.targets)+4)
	lines = append(lines, "Go to", "")
	for i, t := range g.targets {
		marker := "  "
		if i == g.cursor {
			marker = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-*s  %s", marker, catWidth, t.Category, t.Name))
	}
	lines = append(lines, "", "[↑/↓] select   [enter] go   [esc] cancel")
	width := 0
	for _, l := range lines {
		width = max(width, lipgloss.Width(l))
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", width-lipgloss.Width(l))
	}
	return strings.Join(lines, "\n")
}
