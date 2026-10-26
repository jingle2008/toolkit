// Package confirm is the TUI's destructive-action confirmation modal.
//
// The modal names the action about to run, what and where it acts on,
// and a warning, over Cancel and action buttons. A recoverable action
// confirms on y/Y; an irreversible one only on a capital Y, and a
// lowercase y (or enter on its button) redraws the hint as a prompt for
// it. n/N/esc cancel; ←/→/tab move the focus and enter presses the
// focused button. A GPU pool scale-up also takes the instance count,
// typed as digits or pasted. The quit confirmation, shown when quitting
// would cancel running actions, lists them instead.
//
// Model is a value type in the Bubble Tea nested-model style: Update
// returns the updated Model, which the parent stores back, and a Result
// that says whether the operator confirmed or cancelled. The Result is
// returned synchronously, not as a message, so the parent closes the
// modal and starts the action within the same keypress. The Model holds
// no closures and nothing of the parent's: what an action does is the
// parent's, keyed by Result.
package confirm

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/mutation"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/pkg/models"
)

// MaxWidth caps the modal's outer width.
const MaxWidth = 56

// MaxRunningRows caps how many running actions the quit confirmation
// lists.
const MaxRunningRows = 5

// Request is what a confirmation asks about.
type Request struct {
	// Action is the mutation: its tier gates the modal, its labels name
	// it on screen. Zero for the quit confirmation.
	Action  mutation.Action
	Kind    string // the category acted on, for the Kind row
	Target  string // the Name row
	Warning string // what the action does, under the rows
	// Quit marks the quit confirmation; it has no Action.
	Quit bool
	// Pool, when set, makes this a scale-up of it: the modal takes the
	// instance count, starting at the pool's default, with the action
	// button focused so a count typed and entered scales.
	Pool *models.GPUPool
}

// labels is how the modal names each action it confirms — verb and noun
// as an operator reads them, where mutation.Action carries the audit
// record's (scale gpu_pool vs "Scale up GPU pool"). Every action the
// modal can confirm has one (TestLabels_CoverModalActions).
var labels = map[mutation.Action]struct{ verb, noun string }{
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

// Verb is the modal's name for a, e.g. "Scale up"; "" for an action the
// modal doesn't confirm.
func Verb(a mutation.Action) string { return labels[a].verb }

// Noun is the modal's name for what a acts on, e.g. "GPU pool".
func Noun(a mutation.Action) string { return labels[a].noun }

// Verb is the modal's name for r's action, "Quit" for the quit
// confirmation.
func (r Request) Verb() string {
	if r.Quit {
		return "Quit"
	}
	return Verb(r.Action)
}

// Noun is the modal's name for what r acts on.
func (r Request) Noun() string { return Noun(r.Action) }

// Outcome is what a keypress decided.
type Outcome int

// The outcomes: None leaves the modal open.
const (
	None Outcome = iota
	Confirmed
	Cancelled
)

// Result is what Update decided: on Confirmed the parent runs Action
// (with Count instances, for a scale-up); on Cancelled it audits the
// refusal. Either way it closes the modal.
type Result struct {
	Outcome Outcome
	Action  mutation.Action
	Count   int // the scale-up's instance count; 0 otherwise
}

// Model is an open confirmation; the zero Model is none.
type Model struct {
	req Request
	// onAction is the button focus: false (the default) is Cancel.
	onAction bool
	// nudge is set when enter on its button or a lowercase y tries an
	// irreversible action, which only a capital Y may confirm.
	nudge bool
	// amount is the scale-up's count input; nil for every other action.
	// Update replaces it rather than editing it, so copies of a Model
	// never share an input.
	amount *scaleAmount
}

// New opens a confirmation of req.
func New(req Request) Model {
	m := Model{req: req}
	if req.Pool != nil {
		m.amount = newScaleAmount(*req.Pool)
		m.onAction = true
	}
	return m
}

// Request returns what m asks about.
func (m Model) Request() Request { return m.req }

// OnAction reports whether the action button, not Cancel, has the focus.
func (m Model) OnAction() bool { return m.onAction }

// Nudged reports whether the hint is redrawn as a prompt for capital Y.
func (m Model) Nudged() bool { return m.nudge }

// CountText is the scale-up's count as typed; "" for other actions.
func (m Model) CountText() string {
	if m.amount == nil {
		return ""
	}
	return m.amount.input.Value()
}

// Count is the scale-up's entered count, and whether one scale-up may
// add it; false for other actions.
func (m Model) Count() (int, bool) {
	if m.amount == nil {
		return 0, false
	}
	n, problem := m.amount.count()
	return n, problem == ""
}

// scaleAmount is the scale-up's input: how many instances to add to
// pool, the snapshot the modal opened on.
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

// count is the entered count, or what is wrong with it ("" when
// nothing), as the modal says it.
func (a *scaleAmount) count() (n int, problem string) {
	n, err := strconv.Atoi(strings.TrimSpace(a.input.Value()))
	if err != nil {
		return 0, "Enter a whole number of instances, at least 1."
	}
	if err := mutation.CheckPoolScaleAdd(a.pool, n); err != nil {
		return 0, err.Error()
	}
	return n, ""
}

// edits reports whether msg edits a count: a digit, a deletion or a
// paste.
func edits(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		return true
	case tea.KeyPressMsg:
		k := msg.String()
		return k == "backspace" || k == "delete" || (len(k) == 1 && k[0] >= '0' && k[0] <= '9')
	}
	return false
}

// ready reports whether m can be confirmed: always, unless its count is
// invalid.
func (m Model) ready() bool {
	if m.amount == nil {
		return true
	}
	_, ok := m.Count()
	return ok
}

// decide maps a pressed key to confirm/cancel intent for m's tier.
// Recoverable: y/Y confirm. Irreversible: only Y confirms, and a
// lowercase y asks for it (nudge) rather than meaning either, so muscle
// memory neither destroys state nor throws away the confirmation.
// n/N/esc cancel.
func (m Model) decide(s string) (confirm, cancel, nudge bool) {
	cancel = s == "n" || s == "N" || s == "esc"
	if m.irreversible() {
		return s == "Y", cancel, s == "y"
	}
	return s == "y" || s == "Y", cancel, false
}

func (m Model) irreversible() bool { return m.req.Action.Tier == mutation.Irreversible }

// Update resolves a message while the modal is open: a count edit, a
// focus move, or a key that confirms, cancels or nudges. Any other
// message is swallowed so the modal stays put. Quitting the program is
// the parent's: it intercepts its quit key before Update.
func (m Model) Update(msg tea.Msg) (Model, Result) {
	if m.amount != nil && edits(msg) {
		a := *m.amount
		a.input, _ = a.input.Update(msg)
		m.amount = &a
		return m, Result{}
	}
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, Result{}
	}
	k := km.String()
	switch k {
	case "left", "right", "tab", "shift+tab":
		m.onAction = !m.onAction
		return m, Result{}
	case "enter":
		return m.press()
	}
	confirm, cancel, nudge := m.decide(k)
	switch {
	case confirm && m.ready():
		return m, m.result(Confirmed)
	case cancel:
		return m, m.result(Cancelled)
	case nudge:
		m.nudge = true
	}
	return m, Result{}
}

// press is enter on the focused button: Cancel cancels, the action
// button confirms — except an irreversible action, which only a capital
// Y confirms, and a count that isn't valid.
func (m Model) press() (Model, Result) {
	switch {
	case !m.onAction:
		return m, m.result(Cancelled)
	case m.irreversible():
		m.nudge = true
	case m.ready():
		return m, m.result(Confirmed)
	}
	return m, Result{}
}

func (m Model) result(o Outcome) Result {
	r := Result{Outcome: o, Action: m.req.Action}
	if o == Confirmed {
		r.Count, _ = m.Count()
	}
	return r
}

// Styles are the colors the modal is drawn in, taken from the parent's
// theme.
type Styles struct {
	Muted      color.Color // row labels, hints
	Warn       color.Color // the recoverable tier's border, warning and focused button
	OnWarn     color.Color // text on the Warn button
	Danger     color.Color // the irreversible tier's border; an invalid count
	DangerFill color.Color // the focused irreversible button
	OnDanger   color.Color // text on the DangerFill button
	Monochrome bool        // draw the focused button in reverse video
}

// Live is what the modal shows that the parent owns and may change while
// it is open.
type Live struct {
	// KubeContext is the Context row, already badged for production.
	KubeContext string
	// Running are the quit confirmation's running actions.
	Running []string
}

// View draws the modal at most maxWidth columns wide (and at most
// MaxWidth). The irreversible tier is drawn in Danger with a ⚠ title and
// asks for a capital Y; the recoverable tier is drawn in Warn.
func (m Model) View(maxWidth int, st Styles, live Live) string {
	width := min(maxWidth, MaxWidth)
	if m.req.Quit {
		return m.quitView(width, st, live.Running)
	}
	inner := width - 4 // border and padding
	border, fill, onFill, yes := st.Warn, st.Warn, st.OnWarn, "y"
	title := m.req.Verb() + " " + m.req.Noun()
	if m.irreversible() {
		border, fill, onFill, yes = st.Danger, st.DangerFill, st.OnDanger, "Y"
		title = "⚠ " + title
	}

	label := lipgloss.NewStyle().Foreground(st.Muted).Width(9)
	rows := []string{
		label.Render("Kind") + m.req.Kind,
		label.Render("Name") + m.req.Target,
		label.Render("Context") + live.KubeContext,
	}
	warning, warnColor := m.req.Warning, border
	if a := m.amount; a != nil {
		rows = append(rows, label.Render("Add")+a.input.View()+" instances")
		if n, problem := a.count(); problem == "" {
			warning = mutation.PoolScaleNotice(a.pool, n)
		} else {
			warning, warnColor = problem, st.Danger
		}
	}
	body := strings.Join(rows, "\n")
	if warning != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(warnColor).Width(inner).Render(warning)
	}
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, m.buttons(st, fill, onFill))

	hints := yes + " confirm · n/esc cancel · ←/→ select"
	if m.amount != nil {
		hints = "0-9 count · " + yes + "/enter confirm · n/esc cancel"
	}
	if m.irreversible() {
		hints = "Y (shift+y) confirm · n/esc cancel · ←/→ select"
	}
	if m.nudge {
		hints = lipgloss.NewStyle().Foreground(border).Bold(true).
			Render("press capital Y to " + strings.ToLower(m.req.Verb()) + ", or esc to cancel")
	}
	return view.Modal(title, body, hints, border, st.Muted, width)
}

// quitView draws the quit confirmation: the running actions, what
// quitting does to them, and Cancel/Quit buttons.
func (m Model) quitView(width int, st Styles, running []string) string {
	inner := width - 4
	n := len(running)
	shown := running
	if n > MaxRunningRows {
		shown = append(slices.Clone(running[:MaxRunningRows-1]), fmt.Sprintf("… and %d more", n-MaxRunningRows+1))
	}
	lines := make([]string, len(shown))
	for i, r := range shown {
		lines[i] = "  " + view.Truncate(r, inner-2)
	}
	body := strings.Join(lines, "\n") + "\n\n" + lipgloss.NewStyle().Foreground(st.Warn).Width(inner).
		Render("Quitting stops them. A drain resumes when you run it again.")
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, m.buttons(st, st.Warn, st.OnWarn))
	title := fmt.Sprintf("Quit with %d action", n)
	if n > 1 {
		title += "s"
	}
	return view.Modal(title+" running", body, "y/q quit · n/esc cancel · ←/→ select", st.Warn, st.Muted, width)
}

// buttons draws the Cancel and action buttons, the focused one filled
// with fill (reverse video without color, where the fill would vanish).
func (m Model) buttons(st Styles, fill, onFill color.Color) string {
	button := func(text string, focused bool) string {
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case focused && st.Monochrome:
			s = s.Reverse(true).Bold(true)
		case focused:
			s = s.Background(fill).Foreground(onFill).Bold(true)
		}
		return s.Render("[ " + text + " ]")
	}
	return button("Cancel", !m.onAction) + "   " + button(m.req.Verb(), m.onAction)
}
