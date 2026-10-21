package tui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
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
	kind        string // the category acted on, for the Kind row
	auditTarget string
	target      string
	warning     string
	returnView  common.ViewMode
	run         func() tea.Cmd
	// onAction is the button focus: false (the default) is Cancel.
	onAction bool
	// nudge is set when enter on its button or a lowercase y tries an
	// irreversible action, which only a capital Y may confirm; the hint
	// is redrawn as a prompt.
	nudge bool
	// quit marks the quit confirmation, shown when quitting would cancel
	// running actions; it has no mutation.
	quit bool
	// amount is the scale confirmation's instance count; nil for every
	// other action.
	amount *scaleAmount
}

// scaleAmount is the scale confirmation's input: how many instances to
// add to pool, the snapshot the modal opened on.
type scaleAmount struct {
	input textinput.Model
	pool  models.GPUPool
}

// newScaleAmount starts the input at pool's default count.
func newScaleAmount(pool models.GPUPool) *scaleAmount {
	in := textinput.New()
	in.CharLimit = 4
	in.Prompt = ""
	in.SetWidth(5)
	in.SetValue(strconv.Itoa(pool.DefaultScaleAdd()))
	in.Focus()
	return &scaleAmount{input: in, pool: pool}
}

// value is the entered count, and whether it is a whole number ≥ 1.
func (a *scaleAmount) value() (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(a.input.Value()))
	return n, err == nil && n >= 1
}

// update feeds msg to the input when it edits a count — a digit, a
// deletion or a paste — and reports whether it did.
func (a *scaleAmount) update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.PasteMsg:
	case tea.KeyPressMsg:
		k := msg.String()
		if k != "backspace" && k != "delete" && (len(k) != 1 || k[0] < '0' || k[0] > '9') {
			return false
		}
	default:
		return false
	}
	a.input, _ = a.input.Update(msg)
	return true
}

// ready reports whether c can be confirmed: always, unless its count is
// invalid.
func (c confirmOverlay) ready() bool {
	if c.amount == nil {
		return true
	}
	_, ok := c.amount.value()
	return ok
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
func (c confirmOverlay) verb() string {
	if c.quit {
		return "Quit"
	}
	return confirmLabels[c.act].verb
}

// noun is the modal's name for what it acts on, e.g. "GPU pool".
func (c confirmOverlay) noun() string { return confirmLabels[c.act].noun }

// requestConfirm opens the confirmation modal for a destructive action,
// capturing the current view so dismissConfirm can restore it. It returns
// nil: opening the modal issues no command.
//
//nolint:unparam // returns nil so destructive-action handlers can `return m.requestConfirm(...)`
func (m *Model) requestConfirm(c confirmOverlay) tea.Cmd {
	c.returnView = m.viewMode
	c.kind = m.category.String()
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
// overlay's tier. Recoverable: y/Y confirm. Irreversible: only Y
// confirms, and a lowercase y asks for it (nudge) rather than meaning
// either, so muscle memory neither destroys state nor throws away the
// confirmation. n/N/esc cancel.
func (c confirmOverlay) decide(s string) (confirm, cancel, nudge bool) {
	cancel = s == "n" || s == "N" || s == "esc"
	if c.act.Tier == mutation.Irreversible {
		return s == "Y", cancel, s == "y"
	}
	return s == "y" || s == "Y", cancel, false
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

// confirmScale builds the recoverable overlay for the scale key: how many
// instances to add, starting at pool's default, with what that does
// redrawn as it is edited (scaleAmount). The action button starts
// focused, so a count typed and entered scales. run reads the count at
// confirm time.
func (m *Model) confirmScale(itemKey models.ItemKey, pool models.GPUPool) confirmOverlay {
	amount := newScaleAmount(pool)
	return confirmOverlay{
		act:         mutation.ScaleGPUPool,
		target:      itemKeyString(itemKey),
		auditTarget: m.auditName(itemKey),
		amount:      amount,
		onAction:    true,
		run: func() tea.Cmd {
			n, _ := amount.value()
			return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey, n)
		},
	}
}

// updateConfirmView resolves a keypress while the confirmation modal is
// open. Recoverable actions confirm on y/Y; irreversible actions require an
// explicit capital Y, and a lowercase y only redraws the hint as a prompt
// for it. n/esc cancel. ←/→/tab move between the buttons and enter
// presses the focused one, except that enter never confirms an
// irreversible action. Any other key is swallowed so the
// modal stays put. ctrl+c always quits.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirm.amount != nil && m.confirm.amount.update(msg) {
		return m, nil
	}
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(km, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit // a second q quits, even past the quit confirmation
	}
	k := km.String()
	switch k {
	case "left", "right", "tab", "shift+tab":
		m.confirm.onAction = !m.confirm.onAction
		return m, nil
	case "enter":
		return m, m.pressConfirmButton()
	}
	confirm, cancel, nudge := m.confirm.decide(k)
	switch {
	case confirm && m.confirm.ready():
		return m, m.acceptConfirm()
	case cancel:
		m.refuseConfirm()
	case nudge:
		m.confirm.nudge = true
	}
	return m, nil
}

// pressConfirmButton is enter on the focused button: Cancel refuses, the
// action button confirms — except an irreversible action, which only a
// capital Y confirms, and a count that isn't valid.
func (m *Model) pressConfirmButton() tea.Cmd {
	switch {
	case !m.confirm.onAction:
		m.refuseConfirm()
	case m.confirm.act.Tier == mutation.Irreversible:
		m.confirm.nudge = true
	case m.confirm.ready():
		return m.acceptConfirm()
	}
	return nil
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
	border, fill, onFill, yes := p.Warn, p.Warn, p.OnWarn, "y"
	if c.quit {
		return m.quitConfirmView()
	}
	title := c.verb() + " " + c.noun()
	if irreversible {
		border, fill, onFill, yes = p.Danger, p.DangerFill, p.OnAccent, "Y"
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
		label.Render("Kind") + c.kind,
		label.Render("Name") + c.target,
		label.Render("Context") + context,
	}
	warning, warnColor := c.warning, border
	if a := c.amount; a != nil {
		rows = append(rows, label.Render("Add")+a.input.View()+" instances")
		if n, ok := a.value(); ok {
			warning = mutation.PoolScaleNotice(a.pool, n)
		} else {
			warning, warnColor = "Enter a whole number of instances, at least 1.", p.Danger
		}
	}
	body := strings.Join(rows, "\n")
	if warning != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(warnColor).Width(inner).Render(warning)
	}

	buttons := m.confirmButtons(c, fill, onFill)
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, buttons)

	hints := yes + " confirm · n/esc cancel · ←/→ select"
	if c.amount != nil {
		hints = "0-9 count · " + yes + "/enter confirm · n/esc cancel"
	}
	if irreversible {
		hints = "Y (shift+y) confirm · n/esc cancel · ←/→ select"
	}
	if c.nudge {
		hints = lipgloss.NewStyle().Foreground(border).Bold(true).
			Render("press capital Y to " + strings.ToLower(c.verb()) + ", or esc to cancel")
	}
	return m.modal(title, body, hints, border, width)
}

// quit ends the program. With actions running it asks first, since
// quitting cancels them; the confirmation lists them.
func (m *Model) quit() tea.Cmd {
	if len(m.actions) == 0 {
		m.cancelInFlight()
		return tea.Quit
	}
	return m.requestConfirm(confirmOverlay{quit: true, run: func() tea.Cmd {
		m.cancelInFlight()
		return tea.Quit
	}})
}

// quitConfirmViewMaxRows caps how many running actions the quit
// confirmation lists.
const quitConfirmViewMaxRows = 5

// quitConfirmView renders the quit confirmation: the running actions,
// what quitting does to them, and Cancel/Quit buttons.
func (m *Model) quitConfirmView() string {
	c := m.confirm
	p := m.theme.Palette
	width := min(m.modalMaxWidth(), 56)
	inner := width - 4
	running := m.runningSummary()
	if len(running) > quitConfirmViewMaxRows {
		running = append(running[:quitConfirmViewMaxRows-1], fmt.Sprintf("… and %d more", len(running)-quitConfirmViewMaxRows+1))
	}
	for i, r := range running {
		running[i] = "  " + truncateString(r, inner-2)
	}
	body := strings.Join(running, "\n") + "\n\n" + lipgloss.NewStyle().Foreground(p.Warn).Width(inner).
		Render("Quitting stops them. A drain resumes when you run it again.")
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, m.confirmButtons(c, p.Warn, p.OnWarn))
	title := fmt.Sprintf("Quit with %d action", len(m.actions))
	if len(m.actions) > 1 {
		title += "s"
	}
	return m.modal(title+" running", body, "y/q quit · n/esc cancel · ←/→ select", p.Warn, width)
}

// confirmButtons draws c's Cancel and action buttons, the focused one
// filled with fill (reverse video without color, where the fill would
// vanish).
func (m *Model) confirmButtons(c confirmOverlay, fill, onFill color.Color) string {
	button := func(text string, focused bool) string {
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case focused && m.theme.Monochrome:
			s = s.Reverse(true).Bold(true)
		case focused:
			s = s.Background(fill).Foreground(onFill).Bold(true)
		}
		return s.Render("[ " + text + " ]")
	}
	return button("Cancel", !c.onAction) + "   " + button(c.verb(), c.onAction)
}
