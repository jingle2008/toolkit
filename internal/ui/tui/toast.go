// Package tui — toast banner notifications.
//
// A toast is a short message boxed in the bottom-right corner over any
// view mode; up to maxStackedToasts stack upward. Info and warning toasts
// auto-dismiss after toastTTL via tea.Tick, matched by id so an expiry
// only ever removes the toast it was scheduled for. Error toasts stay
// until esc dismisses them, so an error is never missed by looking away.
package tui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// toastTTL is how long an info or warning toast stays before
// auto-dismiss. 8s ≈ enough to read a typical ~80-char message without
// forcing the user to acknowledge it.
const toastTTL = 8 * time.Second

// maxStackedToasts caps how many toasts show at once; a new one pushes
// out the oldest.
const maxStackedToasts = 3

// maxErrorLines caps how many lines an error toast wraps its message to.
const maxErrorLines = 3

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
	if sev == toastError {
		return nil // stays until dismissErrorToasts
	}
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

// hasErrorToast reports whether an error toast is up.
func (m *Model) hasErrorToast() bool {
	t := m.toasts
	isErr := func(o toastState) bool { return o.sev == toastError }
	return (t.active != nil && isErr(*t.active)) || slices.ContainsFunc(t.older, isErr)
}

// dismissErrorToasts removes every error toast; the info and warning
// toasts stay until they expire.
func (m *Model) dismissErrorToasts() {
	t := &m.toasts
	isErr := func(o toastState) bool { return o.sev == toastError }
	t.older = slices.DeleteFunc(t.older, isErr)
	if t.active != nil && isErr(*t.active) {
		t.active = nil
		if n := len(t.older); n > 0 {
			last := t.older[n-1]
			t.active, t.older = &last, t.older[:n-1]
		}
	}
}

// interceptToastDismiss consumes esc to dismiss the error toasts when one
// is up in the list or details view, outside text input and with no
// prefix pending (esc cancels those first) — so esc never
// both dismisses an error and backs out of the view.
func (m *Model) interceptToastDismiss(msg tea.Msg) bool {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok || km.String() != "esc" || !m.hasErrorToast() {
		return false
	}
	if m.viewMode != common.ListView && m.viewMode != common.DetailsView {
		return false
	}
	if m.inputMode != common.NormalInput || m.prefix != "" {
		return false
	}
	m.dismissErrorToasts()
	return true
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

// toastBox draws one toast: a box colored and titled by severity. Info
// and warning messages are truncated to one line; an error's wraps to
// maxErrorLines, with how to dismiss it and where its details are.
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
	inner := max(width-4, 1)
	msg := truncateString(t.msg, inner)
	if t.sev == toastError {
		lines := strings.Split(ansi.Wrap(t.msg, inner, ""), "\n")
		if len(lines) > maxErrorLines {
			lines = lines[:maxErrorLines]
			lines[maxErrorLines-1] = truncateString(lines[maxErrorLines-1]+" …", inner)
		}
		hint := "esc dismiss · " + keys.ToggleLog.Help().Key + " log"
		msg = strings.Join(lines, "\n") + "\n" + lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).Render(hint)
	}
	fit := min(width, max(lipgloss.Width(msg), lipgloss.Width(title)+2)+4)
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	return view.Box(title, msg, border, titleStyle, fit)
}
