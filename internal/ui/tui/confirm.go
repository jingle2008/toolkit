package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
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
//
//nolint:unparam // returns nil so destructive-action handlers can `return m.requestConfirm(...)`
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

// decide maps a pressed key string to confirm/cancel intent for this
// overlay's tier. Recoverable: y/Y confirm; n/N/esc cancel. Irreversible:
// only Y confirms; y/n/N/esc cancel (lowercase y never destroys state).
func (c confirmOverlay) decide(s string) (confirm, cancel bool) {
	switch c.tier {
	case tierIrreversible:
		return s == "Y", s == "y" || s == "n" || s == "N" || s == "esc"
	default: // tierRecoverable
		return s == "y" || s == "Y", s == "n" || s == "N" || s == "esc"
	}
}

// confirmDelete builds the irreversible overlay for the Delete key. The
// action label and warning depend on the category: a DAC is deleted, a GPU
// node's backing instance is terminated. run re-resolves nothing extra —
// deleteItem already re-finds its target by key at execution time.
func (m *Model) confirmDelete(itemKey models.ItemKey) confirmOverlay {
	c := confirmOverlay{
		tier:   tierIrreversible,
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.deleteItem(itemKey) },
	}
	switch m.category {
	case domain.GPUNode:
		c.action, c.kind = "Terminate", "node"
		c.warning = "Boot volume destroyed. Cannot undo."
	default: // DedicatedAICluster
		c.action, c.kind = "Delete", "DAC"
		c.warning = "This is irreversible."
	}
	return c
}

// confirmCordon builds the recoverable overlay for the cordon toggle. The
// run thunk re-resolves the item by key at confirm time so a background
// reload cannot leave it acting on a stale row.
func (m *Model) confirmCordon(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Toggle cordon",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.cordonNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmDrain(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Drain",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.drainNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmReboot(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Reboot",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.rebootNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmScale(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Scale up",
		kind:   "GPU pool",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

// updateConfirmView resolves a keypress while the confirmation modal is
// open. Recoverable actions confirm on y/Y; irreversible actions require an
// explicit capital Y. n/esc cancel; for irreversible, a lowercase y also
// cancels (so muscle-memory never destroys state). Any other key is
// swallowed so the modal stays put. ctrl+c always quits.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(km, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit
	}
	confirm, cancel := m.confirm.decide(km.String())
	if confirm {
		run := m.confirm.run
		m.dismissConfirm()
		return m, run()
	}
	if cancel {
		m.dismissConfirm()
	}
	return m, nil
}
