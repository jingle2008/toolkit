package confirm

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

var styles = Styles{
	Muted:      lipgloss.Color("8"),
	Warn:       lipgloss.Color("3"),
	OnWarn:     lipgloss.Color("0"),
	Danger:     lipgloss.Color("1"),
	DangerFill: lipgloss.Color("9"),
	OnDanger:   lipgloss.Color("15"),
}

var ctx = Live{Rows: [][2]string{{"Context", "ctx-1"}}}

func recoverable() Model {
	return New(Spec{
		Title: "Drain node", Verb: "Drain",
		Rows:    [][2]string{{"Kind", "GPUNode"}, {"Name", "gpu-1"}},
		Warning: "Evicts every pod.",
	})
}

func irreversible() Model {
	return New(Spec{
		Title: "Delete DAC", Verb: "Delete", Tier: Irreversible,
		Rows:    [][2]string{{"Kind", "DedicatedAICluster"}, {"Name", "dac-1"}},
		Warning: "This is irreversible.",
	})
}

// counted is a spec with a count field accepting 1..max, its notice
// naming the count.
func counted(initial string, maxN int) Model {
	return New(Spec{
		Title: "Scale up GPU pool", Verb: "Scale up",
		Rows:    [][2]string{{"Kind", "GPUPool"}, {"Name", "p"}},
		Warning: "unused while the field has a notice",
		Field: &Field{
			Label: "Add", Suffix: "instances", Initial: initial,
			Check: func(text string) (string, string) {
				n, err := strconv.Atoi(text)
				switch {
				case err != nil || n < 1:
					return "", "Enter a whole number, at least 1."
				case n > maxN:
					return "", fmt.Sprintf("%d is more than %d.", n, maxN)
				}
				return fmt.Sprintf("Adds %d.", n), ""
			},
		},
	})
}

func TestNew(t *testing.T) {
	t.Parallel()
	m := recoverable()
	assert.False(t, m.OnAction(), "Cancel has the focus")
	assert.False(t, m.Nudged())
	assert.False(t, m.Quit())
	assert.Empty(t, m.Text())
	assert.Equal(t, "Drain", m.Spec().Verb)

	s := counted("2", 100)
	assert.True(t, s.OnAction(), "a field starts on its action button")
	assert.Equal(t, "2", s.Text())

	q := NewQuit()
	assert.True(t, q.Quit())
	assert.Equal(t, "Quit", q.Spec().Verb)
}

func TestUpdate_Recoverable(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"y", "Y"} {
		_, res := recoverable().Update(press(k))
		assert.Equal(t, Result{Outcome: Confirmed}, res, k)
	}
	for _, k := range []string{"n", "N", "esc"} {
		_, res := recoverable().Update(press(k))
		assert.Equal(t, Result{Outcome: Cancelled}, res, k)
	}
	m, res := recoverable().Update(press("x"))
	assert.Equal(t, None, res.Outcome, "other keys are swallowed")
	assert.False(t, m.Nudged())
	_, res = recoverable().Update(press("5"))
	assert.Equal(t, None, res.Outcome, "digits without a field are swallowed")
	_, res = recoverable().Update(tea.WindowSizeMsg{Width: 1})
	assert.Equal(t, None, res.Outcome, "non-key messages are ignored")
}

func TestUpdate_Irreversible(t *testing.T) {
	t.Parallel()
	m, res := irreversible().Update(press("y"))
	assert.Equal(t, None, res.Outcome, "lowercase y neither confirms nor cancels")
	assert.True(t, m.Nudged())
	_, res = m.Update(press("Y"))
	assert.Equal(t, Confirmed, res.Outcome)

	_, res = irreversible().Update(press("esc"))
	assert.Equal(t, Cancelled, res.Outcome)
}

func TestUpdate_FocusAndEnter(t *testing.T) {
	t.Parallel()
	_, res := recoverable().Update(press("enter"))
	assert.Equal(t, Cancelled, res.Outcome, "enter on Cancel cancels")

	m, _ := recoverable().Update(press("right"))
	assert.True(t, m.OnAction())
	_, res = m.Update(press("enter"))
	assert.Equal(t, Confirmed, res.Outcome)

	m, _ = m.Update(press("tab"))
	m, _ = m.Update(press("left"))
	assert.True(t, m.OnAction(), "each move toggles")

	m, _ = irreversible().Update(press("right"))
	m, res = m.Update(press("enter"))
	assert.Equal(t, None, res.Outcome, "enter never confirms an irreversible action")
	assert.True(t, m.Nudged())
}

// The field takes digits, deletions and pastes; a problem from Check
// blocks confirming, and a confirm carries the text.
func TestUpdate_Field(t *testing.T) {
	t.Parallel()
	m := counted("1", 100)
	orig := m
	m, res := m.Update(press("backspace"))
	assert.Equal(t, None, res.Outcome)
	assert.Empty(t, m.Text())
	assert.Equal(t, "1", orig.Text(), "an edit doesn't reach earlier copies")

	_, res = m.Update(press("y"))
	assert.Equal(t, None, res.Outcome, "a problem blocks y")
	_, res = m.Update(press("enter"))
	assert.Equal(t, None, res.Outcome, "and enter")

	m, _ = m.Update(tea.PasteMsg{Content: "3"})
	m, _ = m.Update(press("x"))
	assert.Equal(t, "3", m.Text(), "letters don't type into it")
	_, res = m.Update(press("enter"))
	assert.Equal(t, Result{Outcome: Confirmed, Text: "3"}, res)
	_, res = m.Update(press("Y"))
	assert.Equal(t, Result{Outcome: Confirmed, Text: "3"}, res)

	_, res = m.Update(press("n"))
	assert.Equal(t, Result{Outcome: Cancelled}, res, "a cancel carries no text")

	for _, d := range "01" {
		m, _ = m.Update(press(string(d)))
	}
	assert.Equal(t, "301", m.Text())
	_, res = m.Update(press("y"))
	assert.Equal(t, None, res.Outcome, "past the check's limit")
}

// A field without a Check accepts any text.
func TestUpdate_FieldWithoutCheck(t *testing.T) {
	t.Parallel()
	m := New(Spec{Verb: "Go", Field: &Field{Label: "N"}})
	_, res := m.Update(press("enter"))
	assert.Equal(t, Result{Outcome: Confirmed}, res)
}

func TestView_Recoverable(t *testing.T) {
	t.Parallel()
	out := ansi.Strip(recoverable().View(120, styles, ctx))
	assert.Contains(t, out, "Drain node")
	assert.NotContains(t, out, "⚠")
	assert.Contains(t, out, "Kind     GPUNode")
	assert.Contains(t, out, "Name     gpu-1")
	assert.Contains(t, out, "Context  ctx-1", "live rows follow the spec's")
	assert.Less(t, strings.Index(out, "Name"), strings.Index(out, "Context"))
	assert.Contains(t, out, "Evicts every pod.")
	assert.Contains(t, out, "[ Cancel ]")
	assert.Contains(t, out, "[ Drain ]")
	assert.Contains(t, out, "y confirm · n/esc cancel · ←/→ select")
	assert.LessOrEqual(t, lipgloss.Width(out), MaxWidth, "capped at MaxWidth")
}

func TestView_Irreversible(t *testing.T) {
	t.Parallel()
	m := irreversible()
	out := ansi.Strip(m.View(120, styles, Live{}))
	assert.Contains(t, out, "⚠ Delete DAC")
	assert.Contains(t, out, "Y (shift+y) confirm · n/esc cancel")

	m, _ = m.Update(press("y"))
	assert.Contains(t, ansi.Strip(m.View(120, styles, Live{})), "press capital Y to delete, or esc to cancel")
	assert.Equal(t, 40, lipgloss.Width(m.View(40, styles, Live{})), "narrower when the screen is")
}

// The field row follows the live rows; its notice replaces the warning,
// and its problem is drawn in Danger instead.
func TestView_Field(t *testing.T) {
	t.Parallel()
	m := counted("2", 100)
	raw := m.View(120, styles, ctx)
	out := ansi.Strip(raw)
	assert.Contains(t, out, "Add      2      instances")
	assert.Less(t, strings.Index(out, "Context"), strings.Index(out, "Add"))
	assert.Contains(t, out, "Adds 2.")
	assert.NotContains(t, out, "unused while")
	assert.Contains(t, out, "0-9 count · y/enter confirm · n/esc cancel")

	m, _ = m.Update(press("backspace"))
	raw = m.View(120, styles, ctx)
	assert.Contains(t, ansi.Strip(raw), "Enter a whole number, at least 1.")
	assert.Contains(t, raw, "\x1b[31mEnter", "a problem is drawn in Danger")

	quiet := New(Spec{Verb: "Go", Warning: "the warning", Field: &Field{Label: "N", Initial: "1"}})
	assert.Contains(t, ansi.Strip(quiet.View(120, styles, Live{})), "the warning", "no notice keeps the warning")
}

func TestView_FocusedButton(t *testing.T) {
	t.Parallel()
	mono := styles
	mono.Monochrome = true
	assert.Contains(t, recoverable().View(120, mono, Live{}), "\x1b[1;7m", "bold reverse without color")
	assert.NotContains(t, recoverable().View(120, styles, Live{}), "\x1b[1;7m")
}

func TestView_Quit(t *testing.T) {
	t.Parallel()
	q := NewQuit()
	out := ansi.Strip(q.View(120, styles, Live{Rows: ctx.Rows, Running: []string{"drain gpu-1"}}))
	assert.Contains(t, out, "Quit with 1 action running")
	assert.Contains(t, out, "  drain gpu-1")
	assert.NotContains(t, out, "Context", "the quit confirmation has no rows")
	assert.Contains(t, out, "[ Quit ]")
	assert.Contains(t, out, "y/q quit · n/esc cancel · ←/→ select")

	running := make([]string, 7)
	for i := range running {
		running[i] = fmt.Sprintf("action %d %s", i, strings.Repeat("x", 80))
	}
	snapshot := append([]string(nil), running...)
	out = ansi.Strip(q.View(120, styles, Live{Running: running}))
	assert.Contains(t, out, "Quit with 7 actions running")
	assert.Contains(t, out, "action 3")
	assert.NotContains(t, out, "action 4")
	assert.Contains(t, out, "… and 3 more")
	assert.Contains(t, out, "x…  │", "long labels are truncated")
	assert.Equal(t, snapshot, running, "the caller's list is left alone")

	_, res := q.Update(press("y"))
	assert.Equal(t, Result{Outcome: Confirmed}, res)
	_, res = q.Update(press("esc"))
	assert.Equal(t, Cancelled, res.Outcome)
}

// Styles carries only colors; keep the zero Styles drawable.
func TestView_ZeroStyles(t *testing.T) {
	t.Parallel()
	var st Styles
	st.Warn = color.Black
	assert.NotEmpty(t, recoverable().View(60, st, Live{}))
}
