package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
)

// levelLabel returns the fixed-width display label for a log level.
func levelLabel(l logging.Level) string {
	switch l {
	case logging.LevelDebug:
		return "DEBUG"
	case logging.LevelInfo:
		return "INFO"
	case logging.LevelWarn:
		return "WARN"
	case logging.LevelError:
		return "ERROR"
	default:
		return "?"
	}
}

// levelStyle returns the color style for a log level.
func (s Styles) levelStyle(l logging.Level) lipgloss.Style {
	switch l {
	case logging.LevelWarn:
		return s.LogWarn
	case logging.LevelError:
		return s.LogError
	case logging.LevelDebug:
		return s.LogDebug
	default:
		return lipgloss.NewStyle()
	}
}

// formatLogEntry renders one entry as a single uncolored line:
// "15:04:05 LEVEL message k=v k=v".
func formatLogEntry(e logging.Entry) string {
	var b strings.Builder
	b.WriteString(e.Time.Format("15:04:05"))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-5s ", levelLabel(e.Level))
	b.WriteString(e.Message)
	for i := 0; i+1 < len(e.Fields); i += 2 {
		fmt.Fprintf(&b, " %v=%v", e.Fields[i], e.Fields[i+1])
	}
	return b.String()
}

// logShown reports whether e passes the level and text filters.
func (m *Model) logShown(e logging.Entry) bool {
	if e.Level < m.log.minLevel {
		return false
	}
	if m.log.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(formatLogEntry(e)), strings.ToLower(m.log.filter))
}

// renderLogEntries builds the log body — one color-coded entry per record
// that passes the filters, oldest first — and counts what it shows of the
// total. Entries wider than the view wrap (word-wrap, with long unbroken
// tokens like OCIDs hard-broken) so the whole line is readable rather than
// clipped with an ellipsis.
func (m *Model) renderLogEntries(width int) (body string, shown, total int) {
	if m.log.store == nil {
		return "(log store unavailable)", 0, 0
	}
	entries := m.log.store.Snapshot()
	if len(entries) == 0 {
		return "(no log entries yet)", 0, 0
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if !m.logShown(e) {
			continue
		}
		style := m.theme.levelStyle(e.Level)
		if width > 0 {
			style = style.Width(width) // wrap to the view width
		}
		lines = append(lines, style.Render(formatLogEntry(e)))
	}
	if len(lines) == 0 {
		return lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).
			Render("no entries match · esc clears the filters"), 0, len(entries)
	}
	return strings.Join(lines, "\n"), len(lines), len(entries)
}

// logView renders the log in the shared frame: the env bar; a breadcrumb
// with the follow/pause state, the level and text filters (or the filter
// input) and the entry count; the entries in a panel; and the log's keys.
// It refreshes from the latest ring snapshot each render and, while the
// user is at the bottom, keeps the newest line in view (auto-follow).
// Scrolling up leaves the offset alone (pause).
func (m *Model) logView() string {
	p := m.theme.Palette
	inner := max(m.viewWidth-2, 1)                   // panel borders
	bodyHeight := max(m.viewHeight-frameChrome-2, 1) // frame lines, panel borders
	m.log.viewport.SetWidth(inner)
	m.log.viewport.SetHeight(bodyHeight)

	follow := m.log.viewport.AtBottom()
	body, shown, total := m.renderLogEntries(max(inner-2, 1))
	m.log.viewport.SetContent(indentLines(body, " "))
	if follow {
		m.log.viewport.GotoBottom()
	}

	titleStyle := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	panel := view.Panel(fmt.Sprintf("Log (%d)", total), m.log.viewport.View(), p.Border, titleStyle, m.viewWidth)
	return lipgloss.JoinVertical(lipgloss.Left, m.envBar(), m.logCrumb(shown, total), panel, m.logFooter())
}

// logCrumb is the log's breadcrumb line: the Log chip, the state (Info
// while following the tail, Warn when paused), the level and text filter
// chips or the filter input, and how many entries show.
func (m *Model) logCrumb(shown, total int) string {
	w := lipgloss.Width
	ctx := m.theme.Context.Render("Log")
	state := m.theme.LogFollowing.Padding(0, 1).Render("following")
	if !m.log.viewport.AtBottom() {
		state = m.theme.LogPaused.Padding(0, 1).Render("PAUSED")
	}
	stats := m.theme.Stats.Render(fmt.Sprintf("[%d/%d]", shown, total))
	left := ctx + " " + state
	var middle string
	if m.log.typing {
		ti := m.log.input
		ti.SetWidth(max(m.viewWidth-w(left)-w(stats)-w(ti.Prompt)-2, 0))
		middle = " " + ti.View()
	} else {
		chip := lipgloss.NewStyle().Foreground(m.theme.Palette.Secondary)
		if m.log.minLevel > logging.LevelDebug {
			middle += " " + chip.Render("⟨≥"+strings.ToLower(levelLabel(m.log.minLevel))+"⟩")
		}
		if m.log.filter != "" {
			middle += " " + chip.Render("⟨/"+m.log.filter+"⟩")
		}
	}
	room := m.viewWidth - w(left) - w(stats)
	if w(middle) > room {
		middle = truncateString(ansi.Strip(middle), max(room, 1))
	}
	return left + middle + strings.Repeat(" ", max(room-w(middle), 0)) + stats
}

// logFooter lists the log's keys, trimmed from the right to fit.
func (m *Model) logFooter() string {
	p := m.theme.Palette
	keyStyle := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(p.Muted)
	pairs := [][2]string{{"/", "filter"}, {"l", "level"}, {"end", "follow"}, {"home", "top"}, {"`", "close"}}
	if m.log.typing {
		pairs = [][2]string{{"enter", "apply"}, {"esc", "clear"}}
	}
	items := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		items = append(items, keyStyle.Render(kv[0])+" "+descStyle.Render(kv[1]))
	}
	for n := len(items); n > 0; n-- {
		if line := " " + strings.Join(items[:n], "  "); lipgloss.Width(line) <= m.viewWidth {
			return line
		}
	}
	return ""
}

// logTickMsg drives periodic re-renders of the log overlay so the live
// tail updates even while the app is otherwise idle.
type logTickMsg struct{}

const logRefreshInterval = 400 * time.Millisecond

// logTickCmd schedules the next log-overlay refresh.
func logTickCmd() tea.Cmd {
	return tea.Tick(logRefreshInterval, func(time.Time) tea.Msg { return logTickMsg{} })
}

// updateLogView handles input while the log is open. While the filter
// input is up, keys type into it: enter keeps the filter, esc clears it.
// Otherwise / opens the input, l cycles the minimum level, home/end jump
// (the viewport keymap lacks them), esc clears the filters and then
// closes, ` closes, and the rest scrolls.
func (m *Model) updateLogView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if ok && m.log.typing {
		return m, m.updateLogInput(km)
	}
	if ok {
		if cmd, handled := m.logKey(km); handled {
			return m, cmd
		}
	}
	vp, cmd := m.log.viewport.Update(msg)
	m.log.viewport = &vp
	return m, cmd
}

// logKey runs the log's own keys and reports whether km was one.
func (m *Model) logKey(km tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(km, keys.Quit):
		m.cancelInFlight()
		return tea.Quit, true
	case key.Matches(km, keys.Back) && (m.log.filter != "" || m.log.minLevel > logging.LevelDebug):
		m.log.filter, m.log.minLevel = "", logging.LevelDebug
		return nil, true
	case key.Matches(km, keys.ToggleLog, keys.Back):
		m.viewMode = m.log.returnView
		return nil, true
	}
	switch km.String() {
	case "/":
		ti := textinput.New()
		ti.Prompt = "/"
		ti.SetValue(m.log.filter)
		m.log.input, m.log.typing = ti, true
		return m.log.input.Focus(), true
	case "l":
		m.log.minLevel = (m.log.minLevel + 1) % (logging.LevelError + 1)
		return nil, true
	case "end":
		m.log.viewport.GotoBottom()
		return nil, true
	case "home":
		m.log.viewport.SetYOffset(0)
		return nil, true
	}
	return nil, false
}

// updateLogInput feeds km to the filter input, filtering as it is typed.
func (m *Model) updateLogInput(km tea.KeyPressMsg) tea.Cmd {
	switch km.String() {
	case "ctrl+c":
		m.cancelInFlight()
		return tea.Quit
	case "enter":
		m.log.typing = false
		return nil
	case "esc":
		m.log.typing, m.log.filter = false, ""
		return nil
	}
	var cmd tea.Cmd
	m.log.input, cmd = m.log.input.Update(km)
	m.log.filter = strings.TrimSpace(m.log.input.Value())
	return cmd
}
