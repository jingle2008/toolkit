package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// maxHistory is the maximum number of entries to keep in navigation history.
const maxHistory = 20

// historyEntry is one visited list: its category and the scope it was
// shown under. The scope is part of what was visited — [ after drilling
// into a node and going to a DAC must return to the node's pods, not to
// the pods of whatever scope is current — so each entry keeps its own
// copy rather than sharing m.scope.
type historyEntry struct {
	category domain.Category
	scope    *domain.Scope
	// view is how the list was left (nil until it is): [ and ] bring the
	// list back as it was seen, not reset.
	view *listView
}

// listView is a list's view state as it was left: its filter (and the
// text the input held), sort, faulty-only mode, and the selected row.
type listView struct {
	filter, input string
	sortColumn    string
	sortAsc       bool
	showFaulty    bool
	cursor        models.ItemKey
}

// saveView records how the current list looks on its history entry, as
// it is left. The entry is matched by category alone: a drill-in or a
// go-to sets the destination's scope just before it navigates.
func (m *Model) saveView() {
	if m.nav.idx < 0 || m.nav.idx >= len(m.nav.history) || m.nav.history[m.nav.idx].category != m.category {
		return
	}
	m.nav.history[m.nav.idx].view = &listView{
		filter:     m.filter,
		input:      m.textInput.Value(),
		sortColumn: m.sortColumn,
		sortAsc:    m.sortAsc,
		showFaulty: m.showFaulty,
		cursor:     itemKeyFrom(m.category, m.selectedRawRow()),
	}
}

// restoreView brings back a list's saved view after a navigation reset
// it; nil (never left) keeps the reset view. The cursor lands on the
// saved row when the rows load.
func (m *Model) restoreView(v *listView) {
	if v == nil {
		return
	}
	m.filter = v.filter
	m.textInput.SetValue(v.input)
	m.sortColumn, m.sortAsc, m.showFaulty = v.sortColumn, v.sortAsc, v.showFaulty
	m.nav.landOnKey = v.cursor
}

// copyScope returns a copy of s, so a history entry never aliases m.scope.
func copyScope(s *domain.Scope) *domain.Scope {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// sameScope reports whether a and b name the same scope (both nil counts).
func sameScope(a, b *domain.Scope) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// pushHistory records a navigation to cat under the current scope,
// discarding any "future" entries if we are not at the end of the list.
// It also enforces a cap of maxHistory entries.
func (m *Model) pushHistory(cat domain.Category) {
	// ignore dups: the same list under the same scope
	if m.nav.idx >= 0 {
		cur := m.nav.history[m.nav.idx]
		if cur.category == cat && sameScope(cur.scope, m.scope) {
			return
		}
	}
	// truncate forward part when user branches
	if m.nav.idx+1 < len(m.nav.history) {
		m.nav.history = m.nav.history[:m.nav.idx+1]
	}
	m.nav.history = append(m.nav.history, historyEntry{category: cat, scope: copyScope(m.scope)})
	m.nav.idx = len(m.nav.history) - 1

	// Cap history size
	if len(m.nav.history) > maxHistory {
		shift := len(m.nav.history) - maxHistory
		m.nav.history = m.nav.history[shift:]
		m.nav.idx -= shift
		if m.nav.idx < 0 {
			m.nav.idx = 0
		}
	}
}

// moveHistory moves idx ±1 (dir = -1 back, +1 forward) and returns a
// tea.Cmd that switches to that entry's category, under its scope,
// WITHOUT recording.
func (m *Model) moveHistory(dir int) tea.Cmd {
	target := m.nav.idx + dir
	if target < 0 || target >= len(m.nav.history) {
		return nil // out of bounds
	}
	m.saveView()
	m.nav.idx = target
	e := m.nav.history[target]
	m.scope = copyScope(e.scope)
	if e.category == m.category {
		// Same list under another scope: updateCategoryCore would treat it
		// as a refresh and keep the old view, so redo what a navigation
		// resets, bring back the entry's view, and recompute the rows.
		m.filter = ""
		m.textInput.Reset()
		m.sortColumn, m.sortAsc, m.showFaulty = common.NameCol, true, false
		m.nav.landOn = nil
		m.restoreView(e.view)
		m.applyRows(nil, nil, false)
		return m.updateRowsAsync()
	}
	cmds := m.updateCategoryNoHist(e.category)
	m.restoreView(e.view)
	return tea.Sequence(cmds...)
}

// syncHistoryScope records the current scope on the current entry, after
// the scope changed in place (esc leaving a scope without navigating).
func (m *Model) syncHistoryScope() {
	if m.nav.idx >= 0 && m.nav.idx < len(m.nav.history) {
		m.nav.history[m.nav.idx].scope = copyScope(m.scope)
	}
}

// dropStaleHistoryScopes drops the entries' scopes that an environment
// switch leaves naming a row of the old environment (scopeSurvives, the
// rule for the current scope): [ must not bring one back (a DAC or node
// that doesn't exist here would list nothing). A definition's scope, and
// a tenant's within the realm, still hold and are kept. The categories
// stay, so [ still retraces where you were.
func (m *Model) dropStaleHistoryScopes(sameRealm bool) {
	for i := range m.nav.history {
		if !scopeSurvives(m.nav.history[i].scope, sameRealm) {
			m.nav.history[i].scope = nil
		}
	}
}
