package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
func levelStyle(l logging.Level) lipgloss.Style {
	switch l {
	case logging.LevelWarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	case logging.LevelError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case logging.LevelDebug:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
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

// renderLogEntries builds the overlay body: one color-coded, width-
// truncated line per entry, oldest first.
func (m *Model) renderLogEntries(width int) string {
	if m.logStore == nil {
		return "(log store unavailable)"
	}
	entries := m.logStore.Snapshot()
	if len(entries) == 0 {
		return "(no log entries yet)"
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = levelStyle(e.Level).Render(truncateString(formatLogEntry(e), width))
	}
	return strings.Join(lines, "\n")
}

// logView renders the full-screen log overlay: a title line showing the
// follow/pause state, the scrollable body, and a key hint footer. It
// refreshes the viewport from the latest ring snapshot each render and,
// while the user is at the bottom, keeps the newest line in view
// (auto-follow). Scrolling up leaves the offset alone (pause).
func (m *Model) logView() string { //nolint:unused // called from Update in Task 6
	width := m.viewWidth
	bodyHeight := m.viewHeight - 2 // title + hint lines
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.logViewport.Width = width
	m.logViewport.Height = bodyHeight

	follow := m.logViewport.AtBottom()
	m.logViewport.SetContent(m.renderLogEntries(width))
	if follow {
		m.logViewport.GotoBottom()
	}

	state := "following"
	if !m.logViewport.AtBottom() {
		state = "PAUSED"
	}
	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("LOG — %s", state))
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Render("↑↓/pgup/pgdn scroll · end follow · home top · ` close")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.logViewport.View(), hint)
}
