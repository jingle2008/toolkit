package confirm

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
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

func recoverable() Model {
	return New(Request{Action: mutation.DrainNode, Kind: "GPUNode", Target: "gpu-1", Warning: "Evicts every pod."})
}

func irreversible() Model {
	return New(Request{Action: mutation.DeleteDAC, Kind: "DedicatedAICluster", Target: "dac-1", Warning: "This is irreversible."})
}

func scale(pool models.GPUPool) Model {
	return New(Request{Action: mutation.ScaleGPUPool, Target: pool.Name, Pool: &pool})
}

// The modal names every action it can confirm — all but set tenant,
// which the edit-tenant form confirms — and a label names one action.
func TestLabels_CoverModalActions(t *testing.T) {
	t.Parallel()
	for _, a := range mutation.Actions() {
		if a == mutation.SetTenant {
			continue
		}
		assert.NotEmpty(t, Verb(a), a.String())
		assert.NotEmpty(t, Noun(a), a.String())
	}
	assert.NotContains(t, labels, mutation.SetTenant)
}

func TestRequest_Labels(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Scale up", Request{Action: mutation.ScaleGPUPool}.Verb())
	assert.Equal(t, "GPU pool", Request{Action: mutation.ScaleGPUPool}.Noun())
	assert.Equal(t, "Quit", Request{Quit: true}.Verb())
}

func TestNew(t *testing.T) {
	t.Parallel()
	m := recoverable()
	assert.False(t, m.OnAction(), "Cancel has the focus")
	assert.False(t, m.Nudged())
	assert.Empty(t, m.CountText())
	_, ok := m.Count()
	assert.False(t, ok, "no count without a pool")
	assert.Equal(t, "gpu-1", m.Request().Target)

	s := scale(models.GPUPool{Name: "p", ID: "x", Size: 4, ActualSize: 2})
	assert.True(t, s.OnAction(), "a scale-up starts on its action button")
	assert.Equal(t, "2", s.CountText())
	n, ok := s.Count()
	require.True(t, ok)
	assert.Equal(t, 2, n)
}

func TestUpdate_Recoverable(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"y", "Y"} {
		_, res := recoverable().Update(press(k))
		assert.Equal(t, Result{Outcome: Confirmed, Action: mutation.DrainNode}, res, k)
	}
	for _, k := range []string{"n", "N", "esc"} {
		_, res := recoverable().Update(press(k))
		assert.Equal(t, Result{Outcome: Cancelled, Action: mutation.DrainNode}, res, k)
	}
	m, res := recoverable().Update(press("x"))
	assert.Equal(t, None, res.Outcome, "other keys are swallowed")
	assert.False(t, m.Nudged())
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
	assert.Equal(t, mutation.DeleteDAC, res.Action)

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

func TestUpdate_ScaleCount(t *testing.T) {
	t.Parallel()
	m := scale(models.GPUPool{Name: "p", ID: "x", Size: 4, ActualSize: 4})
	orig := m
	m, res := m.Update(press("backspace"))
	assert.Equal(t, None, res.Outcome)
	assert.Empty(t, m.CountText())
	assert.Equal(t, "1", orig.CountText(), "an edit doesn't reach earlier copies")

	_, res = m.Update(press("y"))
	assert.Equal(t, None, res.Outcome, "no count, no confirm")
	_, res = m.Update(press("enter"))
	assert.Equal(t, None, res.Outcome)

	m, _ = m.Update(tea.PasteMsg{Content: "3"})
	m, _ = m.Update(press("x"))
	assert.Equal(t, "3", m.CountText(), "letters don't type into it")
	_, res = m.Update(press("enter"))
	assert.Equal(t, Result{Outcome: Confirmed, Action: mutation.ScaleGPUPool, Count: 3}, res)

	_, res = m.Update(press("n"))
	assert.Equal(t, Result{Outcome: Cancelled, Action: mutation.ScaleGPUPool}, res, "a cancel carries no count")

	for _, d := range "01" {
		m, _ = m.Update(press(string(d)))
	}
	assert.Equal(t, "301", m.CountText())
	_, ok := m.Count()
	assert.False(t, ok, "past the one-step limit")
}

func TestView_Recoverable(t *testing.T) {
	t.Parallel()
	out := ansi.Strip(recoverable().View(120, styles, Live{KubeContext: "ctx-1"}))
	assert.Contains(t, out, "Drain node")
	assert.NotContains(t, out, "⚠")
	assert.Contains(t, out, "Kind     GPUNode")
	assert.Contains(t, out, "Name     gpu-1")
	assert.Contains(t, out, "Context  ctx-1")
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

func TestView_Scale(t *testing.T) {
	t.Parallel()
	m := scale(models.GPUPool{Name: "p", ID: "x", Size: 4, ActualSize: 2})
	out := ansi.Strip(m.View(120, styles, Live{}))
	assert.Contains(t, out, "Add      2      instances")
	assert.Contains(t, out, "Grows the instance pool from 2 to 4 instances.")
	assert.Contains(t, out, "0-9 count · y/enter confirm · n/esc cancel")

	m, _ = m.Update(press("backspace"))
	assert.Contains(t, ansi.Strip(m.View(120, styles, Live{})), "Enter a whole number of instances, at least 1.")
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
	q := New(Request{Quit: true})
	out := ansi.Strip(q.View(120, styles, Live{Running: []string{"drain gpu-1"}}))
	assert.Contains(t, out, "Quit with 1 action running")
	assert.Contains(t, out, "  drain gpu-1")
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
	assert.Equal(t, Confirmed, res.Outcome)
}

// Styles carries only colors; keep the zero Styles drawable.
func TestView_ZeroStyles(t *testing.T) {
	t.Parallel()
	var st Styles
	st.Warn = color.Black
	assert.NotEmpty(t, recoverable().View(60, st, Live{}))
}
