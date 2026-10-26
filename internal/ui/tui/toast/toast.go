// Package toast is the TUI's banner-notification overlay.
//
// A toast is a short message boxed in the bottom-right corner over any
// view mode; up to MaxStacked stack upward. Info and warning toasts
// auto-dismiss after TTL via tea.Tick, matched by id so an expiry only
// ever removes the toast it was scheduled for. Error toasts stay until
// DismissErrors removes them, so an error is never missed by looking away.
//
// Model is a value type in the Bubble Tea nested-model style: every
// mutating method returns the updated Model, which the parent stores
// back. The tea.Cmd it returns captures only the toast id, never the
// Model, so it is safe to run off the update goroutine.
package toast

import (
	"image/color"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// TTL is how long an info or warning toast stays before auto-dismiss.
// 8s ≈ enough to read a typical ~80-char message without forcing the
// user to acknowledge it.
const TTL = 8 * time.Second

// MaxStacked caps how many toasts show at once; a new one pushes out the
// oldest info or warning toast, or the oldest error when only errors
// remain.
const MaxStacked = 3

// MaxErrorLines caps how many lines an error toast wraps its message to.
const MaxErrorLines = 3

// Severity picks a toast's color, title and lifetime.
type Severity int

// The severities: Info and Warn fade after TTL; Error stays until
// dismissed.
const (
	Info Severity = iota
	Warn
	Error
)

// Toast is one banner.
type Toast struct {
	Msg  string
	Sev  Severity
	ID   int
	Next string // an error's next step, e.g. "r retry", shown in its hint
}

// ExpireMsg is the tick that removes the toast with id ID, if still up.
type ExpireMsg struct{ ID int }

// Styles are the colors a toast is drawn in, taken from the parent's
// theme, plus the key that opens the log (named in an error's hint).
type Styles struct {
	Info   color.Color // Info border and title
	Warn   color.Color // Warn border and title
	Error  color.Color // Error border and title
	Hint   color.Color // an error's dismiss hint
	LogKey string      // e.g. "`"
}

// Model holds the live toasts. active is the newest, nil when none is
// showing; older holds up to MaxStacked-1 earlier ones still alive,
// oldest first. seq is a monotonic id source that persists across toasts
// so ExpireMsg can match the one it was scheduled for.
type Model struct {
	active *Toast
	older  []Toast
	seq    int
}

// Show raises a toast. Info and warning toasts return the tick that
// expires them; an error returns nil, as it stays until DismissErrors.
func (m Model) Show(msg string, sev Severity) (Model, tea.Cmd) {
	return m.show(Toast{Msg: msg, Sev: sev})
}

// ShowError raises an error toast whose hint names next, the key that
// may fix it ("r retry").
func (m Model) ShowError(msg, next string) (Model, tea.Cmd) {
	return m.show(Toast{Msg: msg, Sev: Error, Next: next})
}

func (m Model) show(t Toast) (Model, tea.Cmd) {
	m.seq++
	t.ID = m.seq
	if m.active != nil {
		m.older = append(slices.Clip(m.older), *m.active)
		for len(m.older) > MaxStacked-1 {
			i := max(slices.IndexFunc(m.older, func(o Toast) bool { return o.Sev != Error }), 0)
			m.older = slices.Delete(m.older, i, i+1)
		}
	}
	m.active = &t
	if t.Sev == Error {
		return m, nil
	}
	id := t.ID
	return m, tea.Tick(TTL, func(time.Time) tea.Msg {
		return ExpireMsg{ID: id}
	})
}

// Update handles the toast's own messages (ExpireMsg); others are
// ignored.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if msg, ok := msg.(ExpireMsg); ok {
		return m.expire(msg.ID), nil
	}
	return m, nil
}

// expire removes the toast with id, if it is still up. When the newest
// expires, the next older one takes its place.
func (m Model) expire(id int) Model {
	if m.active != nil && m.active.ID == id {
		return m.promote()
	}
	m.older = slices.DeleteFunc(slices.Clone(m.older), func(o Toast) bool { return o.ID == id })
	return m
}

// promote drops the newest toast and moves the next older one up.
func (m Model) promote() Model {
	m.active = nil
	if n := len(m.older); n > 0 {
		last := m.older[n-1]
		m.active, m.older = &last, m.older[:n-1]
	}
	return m
}

func isErr(o Toast) bool { return o.Sev == Error }

// HasError reports whether an error toast is up.
func (m Model) HasError() bool {
	return (m.active != nil && isErr(*m.active)) || slices.ContainsFunc(m.older, isErr)
}

// DismissErrors removes every error toast; the info and warning toasts
// stay until they expire.
func (m Model) DismissErrors() Model {
	m.older = slices.DeleteFunc(slices.Clone(m.older), isErr)
	if m.active != nil && isErr(*m.active) {
		return m.promote()
	}
	return m
}

// Visible reports whether any toast is showing.
func (m Model) Visible() bool { return m.active != nil }

// Active returns a copy of the newest toast, nil when none is showing.
func (m Model) Active() *Toast {
	if m.active == nil {
		return nil
	}
	t := *m.active
	return &t
}

// Older returns the earlier toasts still showing, oldest first.
func (m Model) Older() []Toast { return slices.Clone(m.older) }

// View stacks the live toasts, oldest on top and the newest at the
// bottom, each boxed at most width columns wide. It is "" when no toast
// is showing.
func (m Model) View(width int, st Styles) string {
	if m.active == nil {
		return ""
	}
	boxes := make([]string, 0, MaxStacked)
	for _, o := range m.older {
		boxes = append(boxes, box(o, width, st))
	}
	boxes = append(boxes, m.viewActive(width, st))
	return lipgloss.JoinVertical(lipgloss.Right, boxes...)
}

// viewActive boxes the newest toast, "" when none is showing or width
// is not positive.
func (m Model) viewActive(width int, st Styles) string {
	if m.active == nil || width <= 0 {
		return ""
	}
	return box(*m.active, width, st)
}

// box draws one toast: a box colored and titled by severity. Info and
// warning messages are truncated to one line; an error's wraps to
// MaxErrorLines, with how to dismiss it and where its details are.
func box(t Toast, width int, st Styles) string {
	border, title := st.Info, "Info"
	switch t.Sev {
	case Error:
		border, title = st.Error, "✗ Error"
	case Warn:
		border, title = st.Warn, "! Warning"
	case Info:
	}
	inner := max(width-4, 1)
	msg := view.Truncate(t.Msg, inner)
	if t.Sev == Error {
		lines := strings.Split(ansi.Wrap(t.Msg, inner, ""), "\n")
		if len(lines) > MaxErrorLines {
			lines = lines[:MaxErrorLines]
			lines[MaxErrorLines-1] = view.Truncate(lines[MaxErrorLines-1]+" …", inner)
		}
		hint := "esc dismiss · " + st.LogKey + " log"
		if t.Next != "" {
			hint = t.Next + " · " + hint
		}
		msg = strings.Join(lines, "\n") + "\n" + lipgloss.NewStyle().Foreground(st.Hint).Render(hint)
	}
	fit := min(width, max(lipgloss.Width(msg), lipgloss.Width(title)+2)+4)
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	return view.Box(title, msg, border, titleStyle, fit)
}
