// Package tui — toast banner notifications.
//
// A toast is a transient one-line message boxed in the bottom-right corner
// over any view mode; up to maxStackedToasts stack upward. Each
// auto-dismisses after toastTTL via tea.Tick, matched by id so an expiry
// only ever removes the toast it was scheduled for.
package tui

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// toastTTL is how long a toast stays before auto-dismiss. 8s ≈ enough
// to read a typical ~80-char error without forcing the user to
// acknowledge; matches the dwell time k9s and Gmail use for similar
// transient errors.
const toastTTL = 8 * time.Second

// maxStackedToasts caps how many toasts show at once; a new one pushes
// out the oldest.
const maxStackedToasts = 3

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
	m.toasts.seq++
	id := m.toasts.seq
	if m.toasts.active != nil {
		m.toasts.older = append(m.toasts.older, *m.toasts.active)
		if n := len(m.toasts.older) - (maxStackedToasts - 1); n > 0 {
			m.toasts.older = m.toasts.older[n:]
		}
	}
	m.toasts.active = &toastState{msg: msg, sev: sev, id: id}
	return tea.Tick(toastTTL, func(time.Time) tea.Msg {
		return toastExpireMsg{id: id}
	})
}

// handleToastExpireMsg removes the toast msg was scheduled for, if it is
// still up. When the newest expires, the next older one takes its place.
func (m *Model) handleToastExpireMsg(msg toastExpireMsg) {
	t := &m.toasts
	if t.active != nil && t.active.id == msg.id {
		t.active = nil
		if n := len(t.older); n > 0 {
			last := t.older[n-1]
			t.active, t.older = &last, t.older[:n-1]
		}
		return
	}
	t.older = slices.DeleteFunc(t.older, func(o toastState) bool { return o.id == msg.id })
}

// renderToasts stacks the live toasts, oldest on top and the newest at the
// bottom, each boxed at the same width.
func (m *Model) renderToasts() string {
	width := min(60, m.viewWidth-2)
	boxes := make([]string, 0, maxStackedToasts)
	for _, o := range m.toasts.older {
		boxes = append(boxes, m.toastBox(o, width))
	}
	boxes = append(boxes, m.renderToast(width))
	return lipgloss.JoinVertical(lipgloss.Right, boxes...)
}

// renderToast boxes the newest toast at most width columns wide.
func (m *Model) renderToast(width int) string {
	if m.toasts.active == nil || width <= 0 {
		return ""
	}
	return m.toastBox(*m.toasts.active, width)
}

// toastBox draws one toast: a box colored and titled by severity, its
// message truncated to one line so the box never wraps.
func (m *Model) toastBox(t toastState, width int) string {
	p := m.theme.Palette
	border, title := p.Info, "Info"
	switch t.sev {
	case toastError:
		border, title = p.Danger, "✗ Error"
	case toastWarn:
		border, title = p.Warn, "! Warning"
	case toastInfo:
	}
	msg := truncateString(t.msg, max(width-4, 1))
	fit := min(width, max(lipgloss.Width(msg), lipgloss.Width(title)+2)+4)
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	return view.Box(title, msg, border, titleStyle, fit)
}
