package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
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
	if m.historyIdx >= 0 {
		cur := m.history[m.historyIdx]
		if cur.category == cat && sameScope(cur.scope, m.scope) {
			return
		}
	}
	// truncate forward part when user branches
	if m.historyIdx+1 < len(m.history) {
		m.history = m.history[:m.historyIdx+1]
	}
	m.history = append(m.history, historyEntry{category: cat, scope: copyScope(m.scope)})
	m.historyIdx = len(m.history) - 1

	// Cap history size
	if len(m.history) > maxHistory {
		shift := len(m.history) - maxHistory
		m.history = m.history[shift:]
		m.historyIdx -= shift
		if m.historyIdx < 0 {
			m.historyIdx = 0
		}
	}
}

// moveHistory moves idx ±1 (dir = -1 back, +1 forward) and returns a
// tea.Cmd that switches to that entry's category, under its scope,
// WITHOUT recording.
func (m *Model) moveHistory(dir int) tea.Cmd {
	target := m.historyIdx + dir
	if target < 0 || target >= len(m.history) {
		return nil // out of bounds
	}
	m.historyIdx = target
	e := m.history[target]
	m.scope = copyScope(e.scope)
	if e.category == m.category {
		// Same list under another scope: updateCategoryCore would treat it
		// as a refresh and keep the old filter and cursor, so redo what a
		// navigation resets and recompute the rows for the new scope.
		m.filter = ""
		m.textInput.Reset()
		m.applyRows(nil, nil, false)
		return m.updateRowsAsync()
	}
	return tea.Sequence(m.updateCategoryNoHist(e.category)...)
}

// syncHistoryScope records the current scope on the current entry, after
// the scope changed in place (esc leaving a scope without navigating).
func (m *Model) syncHistoryScope() {
	if m.historyIdx >= 0 && m.historyIdx < len(m.history) {
		m.history[m.historyIdx].scope = copyScope(m.scope)
	}
}

// dropStaleHistoryScopes drops the entries' scopes that an environment
// switch leaves naming a row of the old environment (scopeSurvives, the
// rule for the current scope): [ must not bring one back (a DAC or node
// that doesn't exist here would list nothing). A definition's scope, and
// a tenant's within the realm, still hold and are kept. The categories
// stay, so [ still retraces where you were.
func (m *Model) dropStaleHistoryScopes(sameRealm bool) {
	for i := range m.history {
		if !scopeSurvives(m.history[i].scope, sameRealm) {
			m.history[i].scope = nil
		}
	}
}
