package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// prefixState is the pending prefix key ("" when none). The only prefix
// is keys.SortLeader: s then a column's sort key sorts by that column.
type prefixState struct {
	key string
}

// feedPrefix runs the prefix state machine for k in the list view. It
// returns the completed sequence ("s n") once a second key arrives, and
// whether k was used by the machine (and so must not reach the list's
// other handlers). A second key that matches nothing just cancels: the
// caller ignores unknown sequences, and the key is not replayed.
func (m *Model) feedPrefix(k string) (seq string, used bool) {
	if p := m.prefix.key; p != "" {
		m.setPrefix("")
		if k == "esc" {
			return "", true
		}
		return p + " " + k, true
	}
	if k == keys.SortLeader {
		m.setPrefix(k)
		return "", true
	}
	return "", false
}

// setPrefix makes k the pending prefix (or clears it) and redraws the
// column headers, which underline each column's sort key while the sort
// prefix is pending.
func (m *Model) setPrefix(k string) {
	changed := m.prefix.key != k
	m.prefix = prefixState{key: k}
	if changed && m.table != nil {
		m.updateColumns()
	}
}

// handleListSequence runs a completed list-view sequence.
func (m *Model) handleListSequence(seq string) tea.Cmd {
	leader, k, _ := strings.Cut(seq, " ")
	if leader != keys.SortLeader {
		return nil
	}
	if col, ok := keys.SortColumn(m.category, k); ok {
		return m.sortTableByColumn(col)
	}
	return nil
}

// usesListKey reports whether the prefix machine will consume k in the
// list view, so the table must not also see it.
func (m *Model) usesListKey(k string) bool {
	if m.inputMode != common.NormalInput {
		return false
	}
	return m.prefix.key != "" || k == keys.SortLeader
}

// underlineSortKey underlines the first occurrence of key (any case) in
// title. It toggles only the underline attribute, so the header's own
// styling carries on around it.
func underlineSortKey(title, key string) string {
	i := strings.Index(strings.ToLower(title), key)
	if i < 0 || key == "" {
		return title
	}
	return title[:i] + "\x1b[4m" + title[i:i+len(key)] + "\x1b[24m" + title[i+len(key):]
}

// prefixCell is the status-bar marker for a pending prefix, e.g. "s…".
func (m *Model) prefixCell() string {
	if m.prefix.key == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(m.theme.Palette.Primary).Bold(true).
		Render(" " + m.prefix.key + "… ")
}
