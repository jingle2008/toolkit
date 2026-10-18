package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// Prefix keys start a two-key sequence: gg (top) and s⟨column key⟩
// (sort). While one is pending the next key completes or cancels it.
const (
	gotoLeader  = "g"
	prefixDelay = 400 * time.Millisecond
)

// prefixState is the pending prefix key ("" when none). gen discards
// timeouts from earlier prefixes; whichKey is set once the timeout has
// passed and the continuations popup is up.
type prefixState struct {
	key      string
	gen      int
	whichKey bool
}

// prefixTimeoutMsg fires prefixDelay after a prefix key.
type prefixTimeoutMsg struct{ gen int }

// feedPrefix runs the prefix state machine for k. leaders lists the keys
// that may start a sequence in the current view. It returns the completed
// sequence ("g g", "s n") once a second key arrives, whether k was used
// by the machine (and so must not reach the view's other handlers), and
// the timeout command to run when k started a prefix. A second key that
// matches nothing just cancels: the caller ignores unknown sequences, and
// the key is not replayed.
func (m *Model) feedPrefix(k string, leaders ...string) (seq string, used bool, cmd tea.Cmd) {
	if p := m.prefix.key; p != "" {
		m.setPrefix("")
		if k == "esc" {
			return "", true, nil
		}
		return p + " " + k, true, nil
	}
	for _, l := range leaders {
		if k == l {
			m.setPrefix(k)
			gen := m.prefix.gen
			return "", true, tea.Tick(prefixDelay, func(time.Time) tea.Msg { return prefixTimeoutMsg{gen: gen} })
		}
	}
	return "", false, nil
}

// setPrefix makes k the pending prefix (or clears it) and redraws the
// column headers when the sort prefix comes or goes, since they underline
// each column's sort key while it is pending.
func (m *Model) setPrefix(k string) {
	wasSort := m.prefix.key == keys.SortLeader
	m.prefix = prefixState{key: k, gen: m.prefix.gen + 1}
	if wasSort != (k == keys.SortLeader) && m.table != nil {
		m.updateColumns()
	}
}

// handlePrefixTimeout opens the which-key popup if the prefix that
// scheduled msg is still pending. The sort prefix needs no popup: the
// column headers already show its keys.
func (m *Model) handlePrefixTimeout(msg prefixTimeoutMsg) {
	if msg.gen == m.prefix.gen && m.prefix.key == gotoLeader {
		m.prefix.whichKey = true
	}
}

// handleListSequence runs a completed list-view sequence.
func (m *Model) handleListSequence(seq string) tea.Cmd {
	leader, k, _ := strings.Cut(seq, " ")
	switch leader {
	case gotoLeader:
		if k == gotoLeader {
			m.table.GotoTop()
		}
	case keys.SortLeader:
		if col, ok := keys.SortColumn(m.category, k); ok {
			return m.sortTableByColumn(col)
		}
	}
	return nil
}

// listLeaders are the prefix keys the list view accepts.
var listLeaders = []string{gotoLeader, keys.SortLeader}

// usesListKey reports whether the prefix machine will consume k in the
// list view, so the table must not also see it.
func (m *Model) usesListKey(k string) bool {
	if m.inputMode != common.NormalInput {
		return false
	}
	return m.prefix.key != "" || k == gotoLeader || k == keys.SortLeader
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

// whichKeyBox lists what may follow the pending goto prefix.
func (m *Model) whichKeyBox() string {
	p := m.theme.Palette
	k := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	body := k.Render("g") + "  top"
	title := lipgloss.NewStyle().Foreground(p.Focus).Bold(true)
	return view.Box(m.prefix.key+" …", body, p.Focus, title, 0)
}

// prefixCell is the status-bar marker for a pending prefix, e.g. "s…".
func (m *Model) prefixCell() string {
	if m.prefix.key == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(m.theme.Palette.Primary).Bold(true).
		Render(" " + m.prefix.key + "… ")
}
