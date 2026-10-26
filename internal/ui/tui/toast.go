// Package tui — toast banner notifications.
//
// The toast overlay lives in the toast sub-package; this file is the
// parent's thin glue: helpers that store the sub-model back on Model,
// the esc policy that decides when esc dismisses error toasts, and the
// theme-derived styles the sub-model draws in.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/internal/ui/tui/toast"
)

// toastSeverity and its values alias the toast package's, so call sites
// read m.showToast(msg, toastWarn).
type toastSeverity = toast.Severity

const (
	toastInfo  = toast.Info
	toastWarn  = toast.Warn
	toastError = toast.Error
)

func (m *Model) showToast(msg string, sev toastSeverity) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, cmd = m.toasts.Show(msg, sev)
	return cmd
}

// showErrorWithNext raises an error toast whose hint names next, the
// key that may fix it ("r retry").
func (m *Model) showErrorWithNext(msg, next string) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, cmd = m.toasts.ShowError(msg, next)
	return cmd
}

// interceptToastDismiss consumes esc to dismiss the error toasts when one
// is up in the list or details view, outside text input and with no
// prefix pending (esc cancels those first) — so esc never
// both dismisses an error and backs out of the view.
func (m *Model) interceptToastDismiss(msg tea.Msg) bool {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok || km.String() != "esc" || !m.toasts.HasError() {
		return false
	}
	if m.viewMode != common.ListView && m.viewMode != common.DetailsView {
		return false
	}
	if m.inputMode != common.NormalInput || m.prefix != "" {
		return false
	}
	m.toasts = m.toasts.DismissErrors()
	return true
}

// renderToasts stacks the live toasts in the bottom-right corner box,
// at most 60 columns wide.
func (m *Model) renderToasts() string {
	return m.toasts.View(min(60, m.viewWidth-2), m.toastStyles())
}

// toastStyles maps the theme's roles onto the toast overlay.
func (m *Model) toastStyles() toast.Styles {
	p := m.theme.Palette
	return toast.Styles{
		Info:   p.Accent,
		Warn:   p.Warn,
		Error:  p.Danger,
		Hint:   p.Muted,
		LogKey: keys.ToggleLog.Help().Key,
	}
}
