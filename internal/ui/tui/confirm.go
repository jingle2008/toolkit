package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// confirmTier classifies a destructive action by blast radius, which
// determines how the confirmation modal gates it.
type confirmTier int

const (
	// tierRecoverable actions can be undone or retried (cordon, drain,
	// reboot, scale); a single y/N confirms.
	tierRecoverable confirmTier = iota
	// tierIrreversible actions destroy state (delete DAC, terminate node);
	// they require an explicit capital Y behind a DESTRUCTIVE warning.
	tierIrreversible
)

// confirmOverlay holds the state of the destructive-action confirmation
// modal. run is the deferred command; it is invoked only on confirm and
// re-resolves its target at that time so a background reload cannot leave
// it acting on a stale row. returnView restores the prior view on dismiss.
type confirmOverlay struct {
	tier       confirmTier
	action     string
	kind       string
	target     string
	warning    string
	returnView common.ViewMode
	run        func() tea.Cmd
}

// requestConfirm opens the confirmation modal for a destructive action,
// capturing the current view so dismissConfirm can restore it. It returns
// nil: opening the modal issues no command.
func (m *Model) requestConfirm(c confirmOverlay) tea.Cmd {
	c.returnView = m.viewMode
	m.confirm = c
	m.viewMode = common.ConfirmView
	return nil
}

// dismissConfirm closes the modal, restoring the prior view and clearing
// the pending overlay.
func (m *Model) dismissConfirm() {
	m.viewMode = m.confirm.returnView
	m.confirm = confirmOverlay{}
}
