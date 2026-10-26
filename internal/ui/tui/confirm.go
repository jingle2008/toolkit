// Package tui — destructive-action confirmation.
//
// The confirmation modal lives in the confirm sub-package; this file is
// the parent's glue: the builders that decide what each action asks and
// what it runs (closures over the live Model, invoked synchronously on
// confirm), opening and closing the modal, the quit key, dispatching a
// confirm.Result (run the command, or audit the refusal), and the
// theme-derived styles and live context the modal draws.
package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/confirm"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// confirmRequest opens the confirmation modal: what the dialog asks
// about, plus the parent's half — run, the deferred command, and
// auditTarget, the audit record's name for the target. run is invoked
// only on confirm, synchronously on the update goroutine (never inside
// a tea.Cmd), and re-resolves its target at that time so a background
// reload cannot leave it acting on a stale row; count is the scale-up's
// instance count, 0 for other actions. A confirm's audit records come
// from the command run returns; a cancel's from auditTarget.
type confirmRequest struct {
	confirm.Request
	auditTarget string
	run         func(count int) tea.Cmd
}

// confirmPending is the parent's half of the open confirmation: what a
// confirm runs, what a cancel audits, and the view to restore on close.
type confirmPending struct {
	auditTarget string
	run         func(count int) tea.Cmd
	returnView  common.ViewMode
}

// requestConfirm opens the confirmation modal for a destructive action,
// capturing the current view so dismissConfirm can restore it. It returns
// nil: opening the modal issues no command.
//
//nolint:unparam // returns nil so destructive-action handlers can `return m.requestConfirm(...)`
func (m *Model) requestConfirm(c confirmRequest) tea.Cmd {
	c.Kind = m.category.String()
	m.confirm = confirm.New(c.Request)
	m.confirmPending = confirmPending{auditTarget: c.auditTarget, run: c.run, returnView: m.viewMode}
	m.viewMode = common.ConfirmView
	return nil
}

// dismissConfirm closes the modal, restoring the prior view and clearing
// the pending overlay.
func (m *Model) dismissConfirm() {
	m.viewMode = m.confirmPending.returnView
	m.confirm = confirm.Model{}
	m.confirmPending = confirmPending{}
}

// confirmDelete builds the irreversible overlay for the Delete key. The
// action (and so its labels) and warning depend on the category: a DAC is deleted, a GPU
// node's backing instance is terminated, an orphaned InferenceService is
// deleted or — planned as k8s.OrphanFinalized — force-deleted (outcome is
// the plan; empty for the other categories). run re-resolves nothing
// extra — deleteItem already re-finds its target by key at execution time.
func (m *Model) confirmDelete(itemKey models.ItemKey, outcome k8s.OrphanDeleteOutcome) confirmRequest {
	c := confirmRequest{
		Request:     confirm.Request{Target: itemKeyString(itemKey)},
		auditTarget: m.auditName(itemKey),
		run:         func(int) tea.Cmd { return m.deleteItem(itemKey) },
	}
	switch m.category {
	case domain.GPUNode:
		c.Action = mutation.TerminateNode
		c.Warning = "Boot volume destroyed. Cannot undo."
	case domain.InferenceService:
		a := mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized)
		c.Action = a
		c.run = func(int) tea.Cmd { return m.deleteOrphanInferenceService(itemKey, a) }
		c.Warning = "Its DAC is gone. If it stays deleting, run again after 10m to force-delete."
		if outcome == k8s.OrphanFinalized {
			c.Warning = "Stuck deleting: removes its finalizer, skipping the controller's cleanup."
		}
	default: // DedicatedAICluster
		c.Action = mutation.DeleteDAC
		c.Warning = "This is irreversible."
	}
	return c
}

// confirmCordon builds the recoverable overlay for the cordon key: it
// cordons a schedulable node and uncordons a cordoned one, says which,
// and what that does to scheduling.
// The run thunk re-resolves the item by key at confirm time so a
// background reload cannot leave it acting on a stale row; it sets the
// state named here, so a node changed meanwhile ends up as confirmed.
func (m *Model) confirmCordon(itemKey models.ItemKey, item any) confirmRequest {
	a, warning := mutation.CordonNode, "No new pods are scheduled on it; running pods stay."
	if node, ok := item.(*models.GPUNode); ok && node != nil && node.IsSchedulingDisabled {
		a, warning = mutation.UncordonNode, "Pods can be scheduled on it again."
	}
	return confirmRequest{
		Request:     confirm.Request{Action: a, Warning: warning, Target: itemKeyString(itemKey)},
		auditTarget: m.auditName(itemKey),
		run:         func(int) tea.Cmd { return m.cordonNode(findItem(m.dataset, m.category, itemKey), itemKey, a) },
	}
}

// confirmDrain builds the recoverable overlay for the drain key, spelling
// out the drain's steps (k8s.DrainNode) and that it takes a while.
func (m *Model) confirmDrain(itemKey models.ItemKey) confirmRequest {
	return confirmRequest{
		Request: confirm.Request{
			Action:  mutation.DrainNode,
			Warning: "Cordons it, moves single-replica InferenceServices elsewhere, then evicts every pod. Can take tens of minutes.",
			Target:  itemKeyString(itemKey),
		},
		auditTarget: m.auditName(itemKey),
		run:         func(int) tea.Cmd { return m.drainNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

// confirmRestart builds the recoverable overlay for the Restart key: a GPU
// node's instance is soft-reset, an InferenceService is rolling-restarted.
// For an InferenceService the warning surfaces the current Ready state so
// the operator sees whether the endpoint is healthy before disrupting it.
// Other categories don't bind Restart.
func (m *Model) confirmRestart(itemKey models.ItemKey, item any) confirmRequest {
	c := confirmRequest{
		Request:     confirm.Request{Action: mutation.RestartInferenceService, Target: itemKeyString(itemKey)},
		auditTarget: m.auditName(itemKey),
	}
	switch m.category {
	case domain.GPUNode:
		c.Action = mutation.RestartNode
		c.Warning = "Soft-resets the instance; pods running on it are interrupted."
		c.run = func(int) tea.Cmd { return m.restartNode(findItem(m.dataset, m.category, itemKey), itemKey) }
	default: // InferenceService
		c.run = func(int) tea.Cmd { return m.restartInferenceService(findItem(m.dataset, m.category, itemKey), itemKey) }
		if svc, ok := item.(*models.InferenceService); ok && svc != nil {
			c.Warning = "Ready: " + svc.Ready
		}
	}
	return c
}

// requestScale opens the scale confirmation for the pool at itemKey, or
// says why it can't scale: a pool with no OCI instance pool, or one
// whose OCI state isn't loaded, has no instance pool ID to update.
func (m *Model) requestScale(itemKey models.ItemKey, item any) tea.Cmd {
	pool, ok := item.(*models.GPUPool)
	if !ok || pool == nil {
		return nil
	}
	if pool.ID == "" {
		if pool.Status == models.GPUPoolStatusNonexistent {
			return m.showToast(pool.Name+" has no OCI instance pool; nothing to scale", toastWarn)
		}
		return m.showToast(pool.Name+"'s OCI state isn't loaded; refresh and retry", toastWarn)
	}
	return m.requestConfirm(m.confirmScale(itemKey, *pool))
}

// confirmScale builds the recoverable request for the scale key: how
// many instances to add, starting at pool's default, with what that does
// redrawn as it is edited (confirm.Request.Pool). The action button
// starts focused, so a count typed and entered scales. run is handed the
// confirmed count.
func (m *Model) confirmScale(itemKey models.ItemKey, pool models.GPUPool) confirmRequest {
	return confirmRequest{
		Request:     confirm.Request{Action: mutation.ScaleGPUPool, Target: itemKeyString(itemKey), Pool: &pool},
		auditTarget: m.auditName(itemKey),
		run: func(n int) tea.Cmd {
			return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey, n)
		},
	}
}

// updateConfirmView resolves a message while the confirmation modal is
// open: the dialog decides (confirm.Model.Update), and a confirm runs
// the pending command while a cancel is audited, either closing the
// modal. The quit key is the parent's: it always quits, even past the
// quit confirmation.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok && key.Matches(km, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit // a second q quits, even past the quit confirmation
	}
	var res confirm.Result
	m.confirm, res = m.confirm.Update(msg)
	switch res.Outcome {
	case confirm.Confirmed:
		return m, m.acceptConfirm(res.Count)
	case confirm.Cancelled:
		m.refuseConfirm(res.Action)
	case confirm.None:
	}
	return m, nil
}

// acceptConfirm closes the modal and returns the confirmed action's command.
func (m *Model) acceptConfirm(count int) tea.Cmd {
	run := m.confirmPending.run
	m.dismissConfirm()
	return run(count)
}

// refuseConfirm closes the modal without acting. Declining a (an action,
// not the quit confirmation) is audited, as declining the CLI's prompt is.
func (m *Model) refuseConfirm(a mutation.Action) {
	if a.Verb != "" {
		mutation.Audit(m.logger, surfaceTUI, a, m.confirmPending.auditTarget, mutation.PhaseRefused, nil)
	}
	m.dismissConfirm()
}

// confirmView renders the confirmation modal in the theme, with the
// context badged PRODUCTION in production and, for the quit
// confirmation, the running actions.
func (m *Model) confirmView() string {
	live := confirm.Live{KubeContext: m.environment.KubeContext()}
	if m.environment.IsProduction() {
		live.KubeContext = m.theme.Prod.Render("PRODUCTION") + live.KubeContext // the badge pads itself
	}
	if m.confirm.Request().Quit {
		live.Running = m.runningSummary()
	}
	return m.confirm.View(m.modalMaxWidth(), m.confirmStyles(), live)
}

// confirmStyles maps the theme's roles onto the confirmation modal.
func (m *Model) confirmStyles() confirm.Styles {
	p := m.theme.Palette
	return confirm.Styles{
		Muted:      p.Muted,
		Warn:       p.Warn,
		OnWarn:     p.OnWarn,
		Danger:     p.Danger,
		DangerFill: p.DangerFill,
		OnDanger:   p.OnAccent,
		Monochrome: m.theme.Monochrome,
	}
}

// quit ends the program. With actions running it asks first, since
// quitting cancels them; the confirmation lists them.
func (m *Model) quit() tea.Cmd {
	if len(m.actions) == 0 {
		m.cancelInFlight()
		return tea.Quit
	}
	return m.requestConfirm(confirmRequest{Request: confirm.Request{Quit: true}, run: func(int) tea.Cmd {
		m.cancelInFlight()
		return tea.Quit
	}})
}
