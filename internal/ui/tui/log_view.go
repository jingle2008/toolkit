package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
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

// renderLogEntries builds the overlay body: one color-coded entry per
// record, oldest first. Entries wider than the view wrap (word-wrap, with
// long unbroken tokens like OCIDs hard-broken) so the whole line is readable
// rather than clipped with an ellipsis.
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
		style := levelStyle(e.Level)
		if width > 0 {
			style = style.Width(width) // wrap to the view width
		}
		lines[i] = style.Render(formatLogEntry(e))
	}
	return strings.Join(lines, "\n")
}

// logView renders the full-screen log overlay: a title line showing the
// follow/pause state, the scrollable body, and a key hint footer. It
// refreshes the viewport from the latest ring snapshot each render and,
// while the user is at the bottom, keeps the newest line in view
// (auto-follow). Scrolling up leaves the offset alone (pause).
func (m *Model) logView() string {
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

	// Highlighted, full-width title bar — also the visual separator between
	// the header and the scrolling body. The bar color tracks state: teal
	// while following the tail, amber when paused (scrolled up).
	state := "following"
	barColor := lipgloss.Color("24") // teal: live tail
	if !m.logViewport.AtBottom() {
		state = "PAUSED"
		barColor = lipgloss.Color("130") // amber: paused
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("231")).
		Background(barColor).
		Width(width).
		Render(fmt.Sprintf("LOG — %s", state))
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Render("↑↓/pgup/pgdn scroll · end follow · home top · ` close")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.logViewport.View(), hint)
}

// logTickMsg drives periodic re-renders of the log overlay so the live
// tail updates even while the app is otherwise idle.
type logTickMsg struct{}

const logRefreshInterval = 400 * time.Millisecond

// logTickCmd schedules the next log-overlay refresh.
func logTickCmd() tea.Cmd {
	return tea.Tick(logRefreshInterval, func(time.Time) tea.Msg { return logTickMsg{} })
}

// updateLogView handles input while the log overlay is open: close keys,
// quit, the home/end follow controls (the viewport keymap lacks them),
// and otherwise forwards scrolling to the viewport.
func (m *Model) updateLogView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, keys.ToggleLog, keys.Back):
			m.viewMode = m.logReturnView
			return m, nil
		case key.Matches(km, keys.Quit):
			m.cancelInFlight()
			return m, tea.Quit
		}
		switch km.String() {
		case "end":
			m.logViewport.GotoBottom()
			return m, nil
		case "home":
			m.logViewport.SetYOffset(0)
			return m, nil
		}
	}
	vp, cmd := m.logViewport.Update(msg)
	m.logViewport = &vp
	return m, cmd
}
