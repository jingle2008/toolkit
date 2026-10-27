// Package tui — log overlay.
//
// The log itself (filters, key handling, layout and rendering) lives in
// the logview sub-package; this file is the parent's glue: opening the
// log over the list or details and restoring that view on close, the
// quit, refreshing the log's layout from the frame size and theme (on
// open, on each tick, on resize and restyle), and the env bar above it.
package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/logview"
)

// openLog shows the log over the current view, following the tail, and
// arms the live-refresh tick.
func (m *Model) openLog() tea.Cmd {
	m.logReturn = m.viewMode
	m.viewMode = common.LogView
	m.refreshLog()
	return logview.Tick()
}

// refreshLog re-reads the log ring and lays the log out at the frame's
// size in the theme.
func (m *Model) refreshLog() {
	bodyHeight := max(m.viewHeight-frameChrome-2, 1) // frame lines, panel borders
	m.log = m.log.Refresh(m.viewWidth, bodyHeight, m.logStyles())
}

// logStyles are the log's colors from the theme.
func (m *Model) logStyles() logview.Styles {
	t := m.theme
	return logview.Styles{
		Text:      t.Palette.Text,
		Accent:    t.Palette.Accent,
		Muted:     t.Palette.Muted,
		Primary:   t.Palette.Primary,
		Context:   t.Context,
		Following: t.LogFollowing,
		Paused:    t.LogPaused,
		Warn:      t.LogWarn,
		Error:     t.LogError,
		Debug:     t.LogDebug,
	}
}

// handleLogTick refreshes the open log and re-arms the tick; once the log
// has closed the tick stops.
func (m *Model) handleLogTick() tea.Cmd {
	if m.viewMode != common.LogView {
		return nil
	}
	m.refreshLog()
	return logview.Tick()
}

// updateLogView resolves a message while the log is open: the log
// decides (logview.Model.Update); a close restores the view it was
// opened over, a quit quits.
func (m *Model) updateLogView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var res logview.Result
	var cmd tea.Cmd
	m.log, res, cmd = m.log.Update(msg)
	switch res.Outcome {
	case logview.Closed:
		m.viewMode = m.logReturn
	case logview.Quit:
		return m, m.quit()
	case logview.None:
	}
	return m, cmd
}

// logView renders the log in the shared frame: the env bar above the
// log's breadcrumb, panel and keys.
func (m *Model) logView() string {
	return lipgloss.JoinVertical(lipgloss.Left, m.envBar(), m.log.View())
}
