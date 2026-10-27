// Package tui — destructive-action confirmation.
//
// The confirmation modal lives in the confirm sub-package, which knows
// nothing of mutations; this file is the parent's glue: how each action
// is named on screen and how strictly it is gated, the builders that
// decide what each action asks and what it runs (closures over the live
// Model, invoked synchronously on confirm), opening and closing the
// modal, the quit key, dispatching a confirm.Result (run the command, or
// audit the refusal), and the theme-derived styles and live context the
// modal draws.
package tui

import (
	"strconv"
	"strings"

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

// actionLabels is how the TUI names each action it confirms — verb and
// noun as an operator reads them, where mutation.Action carries the
// audit record's (scale gpu_pool vs "Scale up GPU pool"). Every action
// the modal can confirm has one (TestActionLabels_CoverModalActions).
var actionLabels = map[mutation.Action]struct{ verb, noun string }{
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

// actionVerb is the TUI's name for a, e.g. "Scale up"; "" for an action
// the modal doesn't confirm.
func actionVerb(a mutation.Action) string { return actionLabels[a].verb }

// actionNoun is the TUI's name for what a acts on, e.g. "GPU pool".
func actionNoun(a mutation.Action) string { return actionLabels[a].noun }

// confirmTier is how strictly the modal gates a: by its mutation tier.
func confirmTier(a mutation.Action) confirm.Tier {
	if a.Tier == mutation.Irreversible {
		return confirm.Irreversible
	}
	return confirm.Recoverable
}

// confirmRequest is a destructive action awaiting confirmation: the
// action (which names it and sets its tier), its target and warning, an
// optional count field, plus the parent's half — run, the deferred
// command, and auditTarget, the audit record's name for the target. run
// is invoked only on confirm, synchronously on the update goroutine
// (never inside a tea.Cmd), and re-resolves its target at that time so a
// background reload cannot leave it acting on a stale row; its count is
// count(field text) for an action with a field, 0 otherwise. A confirm's
// audit records come from the command run returns; a cancel's from
// auditTarget.
type confirmRequest struct {
	action  mutation.Action
	target  string // the Name row
	warning string
	field   *confirm.Field
	// count parses the field's text into run's count, or says what is
	// wrong with it; the field's Check uses the same rule.
	count       func(text string) (int, string)
	auditTarget string
	run         func(count int) tea.Cmd
}

// confirmPending is the parent's half of the open confirmation: the
// action (zero for the quit confirmation), what a confirm runs, what a
// cancel audits, and the view to restore on close.
type confirmPending struct {
	action      mutation.Action
	count       func(text string) (int, string)
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
	verb := actionVerb(c.action)
	m.openConfirm(confirm.New(confirm.Spec{
		Title:   verb + " " + actionNoun(c.action),
		Verb:    verb,
		Rows:    [][2]string{{"Kind", m.category.String()}, {"Name", c.target}},
		Warning: c.warning,
		Tier:    confirmTier(c.action),
		Field:   c.field,
	}), confirmPending{action: c.action, count: c.count, auditTarget: c.auditTarget, run: c.run})
	return nil
}

// openConfirm shows dialog, with p its parent half; p.returnView is set
// here.
func (m *Model) openConfirm(dialog confirm.Model, p confirmPending) {
	p.returnView = m.viewMode
	m.confirm = dialog
	m.confirmPending = p
	m.viewMode = common.ConfirmView
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
// the plan; empty for the other categories). deleteItem re-finds its
// target by key at execution time; one gone by then is toasted
// (onConfirmedItem).
func (m *Model) confirmDelete(itemKey models.ItemKey, outcome k8s.OrphanDeleteOutcome) confirmRequest {
	c := confirmRequest{
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run: func(int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(any) tea.Cmd { return m.deleteItem(itemKey) })
		},
	}
	switch m.category {
	case domain.GPUNode:
		c.action = mutation.TerminateNode
		c.warning = "Boot volume destroyed. Cannot undo."
	case domain.InferenceService:
		a := mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized)
		c.action = a
		c.run = func(int) tea.Cmd { return m.deleteOrphanInferenceService(itemKey, a) }
		c.warning = "Its DAC is gone. If it stays deleting, run again after 10m to force-delete."
		if outcome == k8s.OrphanFinalized {
			c.warning = "Stuck deleting: removes its finalizer, skipping the controller's cleanup."
		}
	default: // DedicatedAICluster
		c.action = mutation.DeleteDAC
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
func (m *Model) confirmCordon(itemKey models.ItemKey, item any) confirmRequest {
	a, warning := mutation.CordonNode, "No new pods are scheduled on it; running pods stay."
	if node, ok := item.(*models.GPUNode); ok && node != nil && node.IsSchedulingDisabled {
		a, warning = mutation.UncordonNode, "Pods can be scheduled on it again."
	}
	return confirmRequest{
		action:      a,
		warning:     warning,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run: func(int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(item any) tea.Cmd { return m.cordonNode(item, itemKey, a) })
		},
	}
}

// confirmDrain builds the recoverable overlay for the drain key, spelling
// out the drain's steps (k8s.DrainNode) and that it takes a while.
func (m *Model) confirmDrain(itemKey models.ItemKey) confirmRequest {
	return confirmRequest{
		action:      mutation.DrainNode,
		warning:     "Cordons it, moves single-replica InferenceServices elsewhere, then evicts every pod. Can take tens of minutes.",
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		run: func(int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(item any) tea.Cmd { return m.drainNode(item, itemKey) })
		},
	}
}

// confirmRestart builds the recoverable overlay for the Restart key: a GPU
// node's instance is soft-reset, an InferenceService is rolling-restarted.
// For an InferenceService the warning surfaces the current Ready state so
// the operator sees whether the endpoint is healthy before disrupting it.
// Other categories don't bind Restart.
func (m *Model) confirmRestart(itemKey models.ItemKey, item any) confirmRequest {
	c := confirmRequest{
		action:      mutation.RestartInferenceService,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
	}
	switch m.category {
	case domain.GPUNode:
		c.action = mutation.RestartNode
		c.warning = "Soft-resets the instance; pods running on it are interrupted."
		c.run = func(int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(item any) tea.Cmd { return m.restartNode(item, itemKey) })
		}
	default: // InferenceService
		c.run = func(int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(item any) tea.Cmd { return m.restartInferenceService(item, itemKey) })
		}
		if svc, ok := item.(*models.InferenceService); ok && svc != nil {
			c.warning = "Ready: " + svc.Ready
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

// scaleCount parses text as how many instances to add to pool, or says
// what is wrong with it ("" when nothing). The scale field's Check and
// the confirmed run both use it, so they can't disagree.
func scaleCount(pool models.GPUPool, text string) (n int, problem string) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, "Enter a whole number of instances, at least 1."
	}
	if err := mutation.CheckPoolScaleAdd(pool, n); err != nil {
		return 0, err.Error()
	}
	return n, ""
}

// scaleField is the scale-up's count input over pool, a snapshot the
// modal opened on: it starts at the pool's default and says what the
// count does, or what is wrong with it, as it is edited.
func scaleField(pool models.GPUPool) *confirm.Field {
	return &confirm.Field{
		Label:   "Add",
		Suffix:  "instances",
		Initial: strconv.Itoa(pool.DefaultScaleAdd()),
		Check: func(text string) (notice, problem string) {
			n, problem := scaleCount(pool, text)
			if problem != "" {
				return "", problem
			}
			return mutation.PoolScaleNotice(pool, n), ""
		},
	}
}

// confirmScale builds the recoverable request for the scale key: how
// many instances to add (scaleField), with the action button focused, so
// a count typed and entered scales. run is handed the confirmed count.
func (m *Model) confirmScale(itemKey models.ItemKey, pool models.GPUPool) confirmRequest {
	return confirmRequest{
		action:      mutation.ScaleGPUPool,
		target:      itemKeyString(itemKey),
		field:       scaleField(pool),
		count:       func(text string) (int, string) { return scaleCount(pool, text) },
		auditTarget: m.auditName(itemKey),
		run: func(n int) tea.Cmd {
			return m.onConfirmedItem(itemKey, func(item any) tea.Cmd { return m.scaleUpGPUPool(item, itemKey, n) })
		},
	}
}

// onConfirmedItem runs act on the item at itemKey, looked up again as the
// user confirms so a background reload can't leave it acting on a stale
// row. An item a reload removed while the modal was open is toasted as
// gone rather than confirmed into nothing.
func (m *Model) onConfirmedItem(itemKey models.ItemKey, act func(item any) tea.Cmd) tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	if isNilItem(item) {
		return m.showToast(itemKeyString(itemKey)+" is gone; nothing done", toastWarn)
	}
	return act(item)
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
		return m, m.acceptConfirm(res.Text)
	case confirm.Cancelled:
		m.refuseConfirm()
	case confirm.None:
	}
	return m, nil
}

// acceptConfirm closes the modal and returns the confirmed action's
// command, run with the count parsed from the field's text. The dialog
// confirms only a count its Check accepts, which is the same rule, so an
// invalid count here leaves the modal open rather than run.
func (m *Model) acceptConfirm(text string) tea.Cmd {
	p := m.confirmPending
	count := 0
	if p.count != nil {
		n, problem := p.count(text)
		if problem != "" {
			return nil
		}
		count = n
	}
	m.dismissConfirm()
	return p.run(count)
}

// refuseConfirm closes the modal without acting. Declining an action
// (not the quit confirmation) is audited, as declining the CLI's prompt
// is.
func (m *Model) refuseConfirm() {
	if a := m.confirmPending.action; a.Verb != "" {
		mutation.Audit(m.logger, surfaceTUI, a, m.confirmPending.auditTarget, mutation.PhaseRefused, nil)
	}
	m.dismissConfirm()
}

// confirmView renders the confirmation modal in the theme, with the
// context badged PRODUCTION in production and, for the quit
// confirmation, the running actions.
func (m *Model) confirmView() string {
	kubeContext := m.environment.KubeContext()
	if m.environment.IsProduction() {
		kubeContext = m.theme.Prod.Render("PRODUCTION") + kubeContext // the badge pads itself
	}
	live := confirm.Live{Rows: [][2]string{{"Context", kubeContext}}}
	if m.confirm.Quit() {
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
	m.openConfirm(confirm.NewQuit(), confirmPending{run: func(int) tea.Cmd {
		m.cancelInFlight()
		return tea.Quit
	}})
	return nil
}
