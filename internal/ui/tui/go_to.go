package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
)

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
		if relations.ParentRowMissing(m.dataset, t.Category, t.Name) {
			continue
		}
		out = append(out, t)
	}
	return out
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
		return m.openRowPicker("Go to", "go", targets, m.goTo)
	}
}

// goTo navigates to t's category with t as the scope, so the list lands
// on t's row — the same mechanism as the parent jump.
func (m *Model) goTo(t domain.Scope) tea.Cmd {
	m.scope = &t
	return tea.Sequence(m.updateCategory(t.Category)...)
}
