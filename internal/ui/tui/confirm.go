package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// confirmOverlay holds the state of the destructive-action confirmation
// modal. run is the deferred command; it is invoked only on confirm and
// re-resolves its target at that time so a background reload cannot leave
// it acting on a stale row. returnView restores the prior view on dismiss.
// act is the mutation: its tier gates the modal, its labels (verb, noun)
// name it on screen, and with auditTarget it names the audit record a
// cancel writes (a confirm's records come from the command it runs).
type confirmOverlay struct {
	act         mutation.Action
	auditTarget string
	target      string
	warning     string
	returnView  common.ViewMode
	run         func() tea.Cmd
}

// confirmLabels is how the modal names each action it confirms — verb
// and noun as an operator reads them, where mutation.Action carries the
// audit record's (scale gpu_pool vs "Scale up GPU pool"). Every action the
// modal can confirm has one (TestConfirmLabels_CoverModalActions).
var confirmLabels = map[mutation.Action]struct{ verb, noun string }{
	mutation.CordonNode:                  {"Cordon", "node"},
	mutation.UncordonNode:                {"Uncordon", "node"},
	mutation.DrainNode:                   {"Drain", "node"},
	mutation.RestartNode:                 {"Restart", "node"},
	mutation.TerminateNode:               {"Terminate", "node"},
	mutation.ScaleGPUPool:                {"Scale up", "GPU pool"},
	mutation.DeleteDAC:                   {"Delete", "DAC"},
	mutation.RestartInferenceService:     {"Restart", "InferenceService"},
	mutation.DeleteInferenceService:      {"Delete", "orphaned InferenceService"},
	mutation.ForceDeleteInferenceService: {"Force-delete", "orphaned InferenceService"},
}

// verb is the modal's name for the action, e.g. "Scale up".
func (c confirmOverlay) verb() string { return confirmLabels[c.act].verb }

// noun is the modal's name for what it acts on, e.g. "GPU pool".
func (c confirmOverlay) noun() string { return confirmLabels[c.act].noun }

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
	if c.act.Tier == mutation.Irreversible {
		return s == "Y", s == "y" || s == "n" || s == "N" || s == "esc"
	}
	return s == "y" || s == "Y", s == "n" || s == "N" || s == "esc"
}

// confirmDelete builds the irreversible overlay for the Delete key. The
// action (and so its labels) and warning depend on the category: a DAC is deleted, a GPU
// node's backing instance is terminated, an orphaned InferenceService is
// deleted or — planned as k8s.OrphanFinalized — force-deleted (outcome is
// the plan; empty for the other categories). run re-resolves nothing
// extra — deleteItem already re-finds its target by key at execution time.
func (m *Model) confirmDelete(itemKey models.ItemKey, outcome k8s.OrphanDeleteOutcome) confirmOverlay {
	c := confirmOverlay{
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.deleteItem(itemKey) },
	}
	switch m.category {
	case domain.GPUNode:
		c.act = mutation.TerminateNode
		c.warning = "Boot volume destroyed. Cannot undo."
	case domain.InferenceService:
		a := mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized)
		c.act = a
		c.run = func() tea.Cmd { return m.deleteOrphanInferenceService(itemKey, a) }
		c.warning = "Its DAC is gone. If it stays deleting, run again after 10m to force-delete."
		if outcome == k8s.OrphanFinalized {
			c.warning = "Stuck deleting: removes its finalizer, skipping the controller's cleanup."
		}
	default: // DedicatedAICluster
		c.act = mutation.DeleteDAC
		c.warning = "This is irreversible."
	}
	return c
}

// confirmCordon builds the recoverable overlay for the cordon key: it
// cordons a schedulable node and uncordons a cordoned one, and says which.
// The run thunk re-resolves the item by key at confirm time so a
// background reload cannot leave it acting on a stale row; it sets the
// state named here, so a node changed meanwhile ends up as confirmed.
func (m *Model) confirmCordon(itemKey models.ItemKey, item any) confirmOverlay {
	a := mutation.CordonNode
	if node, ok := item.(*models.GPUNode); ok && node != nil && node.IsSchedulingDisabled {
		a = mutation.UncordonNode
	}
	return confirmOverlay{
		act:         a,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.cordonNode(findItem(m.dataset, m.category, itemKey), itemKey, a) },
	}
}

func (m *Model) confirmDrain(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		act:         mutation.DrainNode,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.drainNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

// confirmRestart builds the recoverable overlay for the Restart key: a GPU
// node's instance is soft-reset, an InferenceService is rolling-restarted.
// For an InferenceService the warning surfaces the current Ready state so
// the operator sees whether the endpoint is healthy before disrupting it.
// Other categories don't bind Restart.
func (m *Model) confirmRestart(itemKey models.ItemKey, item any) confirmOverlay {
	c := confirmOverlay{
		act:         mutation.RestartInferenceService,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
	}
	switch m.category {
	case domain.GPUNode:
		c.act = mutation.RestartNode
		c.run = func() tea.Cmd { return m.restartNode(findItem(m.dataset, m.category, itemKey), itemKey) }
	default: // InferenceService
		c.run = func() tea.Cmd { return m.restartInferenceService(findItem(m.dataset, m.category, itemKey), itemKey) }
		if svc, ok := item.(*models.InferenceService); ok && svc != nil {
			c.warning = "Ready: " + svc.Ready
		}
	}
	return c
}

func (m *Model) confirmScale(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		act:         mutation.ScaleGPUPool,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

// updateConfirmView resolves a keypress while the confirmation modal is
// open. Recoverable actions confirm on y/Y; irreversible actions require an
// explicit capital Y. n/esc cancel; for irreversible, a lowercase y also
// cancels (so muscle-memory never destroys state). Any other key is
// swallowed so the modal stays put. ctrl+c always quits.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
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
		if c := m.confirm; c.act.Verb != "" {
			// Declining is audited, as declining the CLI's prompt is.
			mutation.Audit(m.logger, surfaceTUI, c.act, c.auditTarget, mutation.PhaseRefused, nil)
		}
		m.dismissConfirm()
	}
	return m, nil
}

// confirmView renders the confirmation modal body. The irreversible tier
// leads with a DESTRUCTIVE banner, shows the warning line, and asks for a
// capital Y; the recoverable tier is a plain y/N prompt.
func (m *Model) confirmView() string {
	c := m.confirm
	var b strings.Builder
	if c.act.Tier == mutation.Irreversible {
		b.WriteString("⚠ DESTRUCTIVE\n\n")
		fmt.Fprintf(&b, "%s %s  %s\n", c.verb(), c.noun(), c.target)
		if c.warning != "" {
			fmt.Fprintf(&b, "%s\n", c.warning)
		}
		b.WriteString("\nPress Y to confirm   n/esc cancel")
		return b.String()
	}
	fmt.Fprintf(&b, "Confirm %s\n\n", strings.ToLower(c.verb()))
	fmt.Fprintf(&b, "%s %s  %s ?\n", c.verb(), c.noun(), c.target)
	if c.warning != "" {
		fmt.Fprintf(&b, "%s\n", c.warning)
	}
	b.WriteString("\n[y] confirm   [n/esc] cancel")
	return b.String()
}
