// Package confirm is the TUI's confirmation modal: a generic dialog the
// parent describes with a Spec.
//
// The modal shows a title, label/value rows and a warning over Cancel
// and action buttons. A Recoverable spec confirms on y/Y; an
// Irreversible one only on a capital Y, and a lowercase y (or enter on
// its button) redraws the hint as a prompt for it. n/N/esc cancel;
// ←/→/tab move the focus and enter presses the focused button. A spec
// may carry one count Field, typed as digits or pasted, whose validator
// the parent supplies; confirming waits on it. The quit confirmation
// (NewQuit), shown when quitting would cancel running actions, lists
// them instead.
//
// Model is a value type in the Bubble Tea nested-model style: Update
// returns the updated Model, which the parent stores back, and a Result
// that says whether the operator confirmed or cancelled. The Result is
// returned synchronously, not as a message, so the parent closes the
// modal and starts the action within the same keypress. The package
// knows nothing of what is confirmed: what an action is called, how
// strict it is and what it does are the parent's.
package confirm

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// MaxWidth caps the modal's outer width.
const MaxWidth = 56

// MaxRunningRows caps how many running actions the quit confirmation
// lists.
const MaxRunningRows = 5

// Tier is how strictly a confirmation is gated.
type Tier int

// The tiers: Recoverable confirms on y/Y and is drawn in Warn;
// Irreversible confirms only on a capital Y, nudges on a lowercase y,
// and is drawn in Danger under a ⚠ title.
const (
	Recoverable Tier = iota
	Irreversible
)

// Spec describes a confirmation.
type Spec struct {
	Title   string      // e.g. "Scale up GPU pool"; the modal adds ⚠ for Irreversible
	Verb    string      // the action button, e.g. "Scale up"; lowercased in the nudge
	Rows    [][2]string // static label/value rows, e.g. {"Kind", "GPUPool"}
	Warning string      // what the action does, under the rows
	Tier    Tier
	// Field, when set, adds a count input after the rows, with the action
	// button focused so a count typed and entered confirms.
	Field *Field
}

// Field is a spec's count input: digits, a deletion or a paste edit it.
type Field struct {
	Label   string // the row label, e.g. "Add"
	Suffix  string // after the input, e.g. "instances"
	Initial string // the input's starting text
	// Check validates the text as typed: a problem (drawn in Danger, in
	// place of the warning) blocks confirming; otherwise a non-empty
	// notice replaces the warning. It must be pure — it runs on every
	// View and keypress. Nil accepts anything.
	Check func(text string) (notice, problem string)
}

// Outcome is what a keypress decided.
type Outcome int

// The outcomes: None leaves the modal open.
const (
	None Outcome = iota
	Confirmed
	Cancelled
)

// Result is what Update decided: on Confirmed the parent runs its
// pending action, with Text the field's text ("" without a field); on
// Cancelled it audits the refusal. Either way it closes the modal.
type Result struct {
	Outcome Outcome
	Text    string
}

// Model is an open confirmation; the zero Model is none.
type Model struct {
	spec Spec
	quit bool
	// onAction is the button focus: false (the default) is Cancel.
	onAction bool
	// nudge is set when enter on its button or a lowercase y tries an
	// irreversible action, which only a capital Y may confirm.
	nudge bool
	// input is the field's input; nil without a field. Update replaces
	// it rather than editing it, so copies of a Model never share one.
	input *textinput.Model
}

// New opens a confirmation described by spec.
func New(spec Spec) Model {
	m := Model{spec: spec}
	if f := spec.Field; f != nil {
		in := textinput.New()
		in.CharLimit = 4
		in.Prompt = ""
		in.SetWidth(5)
		in.SetValue(f.Initial)
		in.Focus()
		m.input = &in
		m.onAction = true
	}
	return m
}

// NewQuit opens the quit confirmation; View lists the running actions.
func NewQuit() Model {
	return Model{spec: Spec{Verb: "Quit"}, quit: true}
}

// Spec returns what m was opened with.
func (m Model) Spec() Spec { return m.spec }

// Quit reports whether m is the quit confirmation.
func (m Model) Quit() bool { return m.quit }

// OnAction reports whether the action button, not Cancel, has the focus.
func (m Model) OnAction() bool { return m.onAction }

// Nudged reports whether the hint is redrawn as a prompt for capital Y.
func (m Model) Nudged() bool { return m.nudge }

// Text is the field's text as typed; "" without a field.
func (m Model) Text() string {
	if m.input == nil {
		return ""
	}
	return m.input.Value()
}

// check runs the field's validator over its text.
func (m Model) check() (notice, problem string) {
	if m.input == nil || m.spec.Field.Check == nil {
		return "", ""
	}
	return m.spec.Field.Check(m.Text())
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

// ready reports whether m can be confirmed: unless its field's check
// reports a problem.
func (m Model) ready() bool {
	_, problem := m.check()
	return problem == ""
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

func (m Model) irreversible() bool { return m.spec.Tier == Irreversible }

// Update resolves a message while the modal is open: a field edit, a
// focus move, or a key that confirms, cancels or nudges. Any other
// message is swallowed so the modal stays put. Quitting the program is
// the parent's: it intercepts its quit key before Update.
func (m Model) Update(msg tea.Msg) (Model, Result) {
	if m.input != nil && edits(msg) {
		in := *m.input
		in, _ = in.Update(msg)
		m.input = &in
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
// Y confirms, and a field whose check reports a problem.
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
	r := Result{Outcome: o}
	if o == Confirmed {
		r.Text = m.Text()
	}
	return r
}

// Styles are the colors the modal is drawn in, taken from the parent's
// theme.
type Styles struct {
	Muted      color.Color // row labels, hints
	Warn       color.Color // the recoverable tier's border, warning and focused button
	OnWarn     color.Color // text on the Warn button
	Danger     color.Color // the irreversible tier's border; a field's problem
	DangerFill color.Color // the focused irreversible button
	OnDanger   color.Color // text on the DangerFill button
	Monochrome bool        // draw the focused button in reverse video
}

// Live is what the modal shows that the parent owns and may change while
// it is open.
type Live struct {
	// Rows follow the spec's rows, e.g. a Context row badged for
	// production.
	Rows [][2]string
	// Running are the quit confirmation's running actions.
	Running []string
}

// View draws the modal at most maxWidth columns wide (and at most
// MaxWidth): the spec's rows, then live's, then the field. The
// irreversible tier is drawn in Danger with a ⚠ title and asks for a
// capital Y; the recoverable tier is drawn in Warn.
func (m Model) View(maxWidth int, st Styles, live Live) string {
	width := min(maxWidth, MaxWidth)
	if m.quit {
		return m.quitView(width, st, live.Running)
	}
	inner := width - 4 // border and padding
	border, fill, onFill, yes := st.Warn, st.Warn, st.OnWarn, "y"
	title := m.spec.Title
	if m.irreversible() {
		border, fill, onFill, yes = st.Danger, st.DangerFill, st.OnDanger, "Y"
		title = "⚠ " + title
	}

	label := lipgloss.NewStyle().Foreground(st.Muted).Width(9)
	rows := make([]string, 0, len(m.spec.Rows)+len(live.Rows)+1)
	for _, r := range slices.Concat(m.spec.Rows, live.Rows) {
		rows = append(rows, label.Render(r[0])+r[1])
	}
	warning, warnColor := m.spec.Warning, border
	if f := m.spec.Field; f != nil {
		rows = append(rows, label.Render(f.Label)+m.input.View()+" "+f.Suffix)
		switch notice, problem := m.check(); {
		case problem != "":
			warning, warnColor = problem, st.Danger
		case notice != "":
			warning = notice
		}
	}
	body := strings.Join(rows, "\n")
	if warning != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(warnColor).Width(inner).Render(warning)
	}
	body += "\n\n" + lipgloss.PlaceHorizontal(inner, lipgloss.Center, m.buttons(st, fill, onFill))

	hints := yes + " confirm · n/esc cancel · ←/→ select"
	if m.input != nil {
		hints = "0-9 count · " + yes + "/enter confirm · n/esc cancel"
	}
	if m.irreversible() {
		hints = "Y (shift+y) confirm · n/esc cancel · ←/→ select"
	}
	if m.nudge {
		hints = lipgloss.NewStyle().Foreground(border).Bold(true).
			Render("press capital Y to " + strings.ToLower(m.spec.Verb) + ", or esc to cancel")
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
	return button("Cancel", !m.onAction) + "   " + button(m.spec.Verb, m.onAction)
}
