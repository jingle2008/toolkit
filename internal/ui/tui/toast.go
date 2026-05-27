// Package tui — toast banner notifications.
//
// A toast is a transient one-line message rendered over any view mode.
// It auto-dismisses after toastTTL via tea.Tick. The id guards against
// stale expiry messages clearing a newer toast.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// toastTTL is how long a toast stays before auto-dismiss. 8s ≈ enough
// to read a typical ~80-char error without forcing the user to
// acknowledge; matches the dwell time k9s and Gmail use for similar
// transient errors.
const toastTTL = 8 * time.Second

type toastSeverity int

const (
	toastInfo toastSeverity = iota
	toastWarn
	toastError
)

type toastState struct {
	msg string
	sev toastSeverity
	id  int
}

type toastExpireMsg struct{ id int }

func (m *Model) showToast(msg string, sev toastSeverity) tea.Cmd {
	m.toastSeq++
	id := m.toastSeq
	m.toast = &toastState{msg: msg, sev: sev, id: id}
	return tea.Tick(toastTTL, func(time.Time) tea.Msg {
		return toastExpireMsg{id: id}
	})
}

func (m *Model) handleToastExpireMsg(msg toastExpireMsg) {
	if m.toast != nil && m.toast.id == msg.id {
		m.toast = nil
	}
}

func (m *Model) renderToast(width int) string {
	if m.toast == nil || width <= 0 {
		return ""
	}
	var style lipgloss.Style
	switch m.toast.sev {
	case toastError:
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("160")).
			Foreground(lipgloss.Color("231")).
			Bold(true)
	case toastWarn:
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("214")).
			Foreground(lipgloss.Color("16"))
	default:
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(lipgloss.Color("231"))
	}
	style = style.Padding(0, 1).Width(width)
	// Width(width) includes padding; truncate the message to avoid
	// lipgloss soft-wrapping into a multi-line banner.
	return style.Render(truncateString(m.toast.msg, width-2))
}
