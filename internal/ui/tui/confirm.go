package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	// onAction is the button focus: false (the default) is Cancel.
	onAction bool
	// nudge is set when enter lands on an irreversible action's button,
	// which only a capital Y may confirm; the hint is redrawn as a prompt.
	nudge bool
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
// cordons a schedulable node and uncordons a cordoned one, says which,
// and what that does to scheduling.
// The run thunk re-resolves the item by key at confirm time so a
// background reload cannot leave it acting on a stale row; it sets the
// state named here, so a node changed meanwhile ends up as confirmed.
func (m *Model) confirmCordon(itemKey models.ItemKey, item any) confirmOverlay {
	a, warning := mutation.CordonNode, "No new pods are scheduled on it; running pods stay."
	if node, ok := item.(*models.GPUNode); ok && node != nil && node.IsSchedulingDisabled {
		a, warning = mutation.UncordonNode, "Pods can be scheduled on it again."
	}
	return confirmOverlay{
		act:         a,
		warning:     warning,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.cordonNode(findItem(m.dataset, m.category, itemKey), itemKey, a) },
	}
}

// confirmDrain builds the recoverable overlay for the drain key, spelling
// out the drain's steps (k8s.DrainNode) and that it takes a while.
func (m *Model) confirmDrain(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		act:         mutation.DrainNode,
		warning:     "Cordons it, moves single-replica InferenceServices elsewhere, then evicts every pod. Can take tens of minutes.",
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
		c.warning = "Soft-resets the instance; pods running on it are interrupted."
		c.run = func() tea.Cmd { return m.restartNode(findItem(m.dataset, m.category, itemKey), itemKey) }
	default: // InferenceService
		c.run = func() tea.Cmd { return m.restartInferenceService(findItem(m.dataset, m.category, itemKey), itemKey) }
		if svc, ok := item.(*models.InferenceService); ok && svc != nil {
			c.warning = "Ready: " + svc.Ready
		}
	}
	return c
}

// confirmScale builds the recoverable overlay for the scale key, saying
// how far the pool grows: oci.IncreasePoolSize raises the instance pool
// to the pool's configured size, and does nothing when it is there.
func (m *Model) confirmScale(itemKey models.ItemKey) confirmOverlay {
	c := confirmOverlay{
		act:         mutation.ScaleGPUPool,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run:         func() tea.Cmd { return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
	if m.dataset == nil {
		return c
	}
	if pool, ok := findItem(m.dataset, m.category, itemKey).(*models.GPUPool); ok && pool != nil {
		if pool.ActualSize >= pool.Size {
			c.warning = fmt.Sprintf("Already at its configured size (%d); nothing to do.", pool.Size)
		} else {
			c.warning = fmt.Sprintf("Grows the instance pool from %d to %d instances.", pool.ActualSize, pool.Size)
		}
	}
	return c
}

// updateConfirmView resolves a keypress while the confirmation modal is
// open. Recoverable actions confirm on y/Y; irreversible actions require an
// explicit capital Y. n/esc cancel; for irreversible, a lowercase y also
// cancels (so muscle-memory never destroys state). ←/→/tab move between
// the buttons and enter presses the focused one, except that enter never
// confirms an irreversible action. Any other key is swallowed so the
// modal stays put. ctrl+c always quits.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(km, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit
	}
	k := km.String()
	switch k {
	case "left", "right", "tab", "shift+tab":
		m.confirm.onAction = !m.confirm.onAction
		return m, nil
	case "enter":
		switch {
		case !m.confirm.onAction:
			m.refuseConfirm()
		case m.confirm.act.Tier == mutation.Irreversible:
			m.confirm.nudge = true
		default:
			return m, m.acceptConfirm()
		}
		return m, nil
	}
	confirm, cancel := m.confirm.decide(k)
	if confirm {
		return m, m.acceptConfirm()
	}
	if cancel {
		m.refuseConfirm()
	}
	return m, nil
}

// acceptConfirm closes the modal and returns the confirmed action's command.
func (m *Model) acceptConfirm() tea.Cmd {
	run := m.confirm.run
	m.dismissConfirm()
	return run()
}

// refuseConfirm closes the modal without acting. Declining is audited, as
// declining the CLI's prompt is.
func (m *Model) refuseConfirm() {
	if c := m.confirm; c.act.Verb != "" {
		mutation.Audit(m.logger, surfaceTUI, c.act, c.auditTarget, mutation.PhaseRefused, nil)
	}
	m.dismissConfirm()
}

// confirmView renders the confirmation modal: what is about to happen, to
// what and where, the warning, and Cancel/action buttons. The irreversible
// tier is drawn in Danger with a ⚠ title and asks for a capital Y; the
// recoverable tier is drawn in Warn.
func (m *Model) confirmView() string {
	c := m.confirm
	p := m.theme.Palette
	irreversible := c.act.Tier == mutation.Irreversible
	border, onBorder, yes := p.Warn, p.OnWarn, "y"
	title := c.verb() + " " + c.noun()
	if irreversible {
		border, onBorder, yes = p.Danger, p.OnAccent, "Y"
		title = "⚠ " + title
	}
	width := min(m.modalMaxWidth(), 56)
	inner := width - 4 // border and padding

	label := lipgloss.NewStyle().Foreground(p.Muted).Width(9)
	context := m.environment.KubeContext()
	if m.environment.IsProduction() {
		context = m.theme.Prod.Render("PRODUCTION") + context // the badge pads itself
	}
	rows := []string{
		label.Render("Kind") + c.noun(),
		label.Render("Name") + c.target,
		label.Render("Context") + context,
	}
	body := strings.Join(rows, "\n")
	if c.warning != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(border).Width(inner).Render(c.warning)
	}

	button := func(text string, focused bool) string {
		s := lipgloss.NewStyle().Padding(0, 1)
		if focused {
			s = s.Background(border).Foreground(onBorder).Bold(true)
		}
		return s.Render("[ " + text + " ]")
	}
	buttons := button("Cancel", !c.onAction) + "   " + button(c.verb(), c.onAction)
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, buttons)

	hints := yes + " confirm · n/esc cancel · ←/→ select"
	if irreversible {
		hints = "Y (shift+y) confirm · y/n/esc cancel · ←/→ select"
	}
	if c.nudge {
		hints = lipgloss.NewStyle().Foreground(border).Bold(true).
			Render("press capital Y to " + strings.ToLower(c.verb()) + ", or esc to cancel")
	}
	return m.modal(title, body, hints, border, width)
}
