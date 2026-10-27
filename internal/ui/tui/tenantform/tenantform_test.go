package tenantform

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ocid = "ocid1.tenancy.oc1..abc"

var styles = Styles{Accent: color.White, Muted: color.Black, Danger: color.White}

func kp(k string) tea.KeyPressMsg {
	switch k {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+o":
		return tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

// press sends each key in turn, returning the model and the last result.
func press(t *testing.T, m Model, ks ...string) (Model, Result) {
	t.Helper()
	var res Result
	for _, k := range ks {
		m, res, _ = m.Update(kp(k), 80)
	}
	return m, res
}

func TestNew(t *testing.T) {
	t.Parallel()
	m := New(ocid, "abc")
	assert.True(t, m.Visible())
	assert.Equal(t, ocid, m.OCID())
	assert.Equal(t, "abc", m.TenantID())
	assert.Equal(t, FieldName, m.Focus())
	assert.True(t, m.Internal(), "internal by default")
	assert.Empty(t, m.Name())
	assert.Empty(t, m.Note())
	assert.Empty(t, m.NameErr())
	assert.False(t, Model{}.Visible(), "the zero Model is none")
}

func TestCycleFocus(t *testing.T) {
	t.Parallel()
	m := New(ocid, "abc")
	m = m.CycleFocus(1)
	assert.Equal(t, FieldInternal, m.Focus())
	m = m.CycleFocus(1)
	assert.Equal(t, FieldNote, m.Focus())
	m = m.CycleFocus(1)
	assert.Equal(t, FieldName, m.Focus(), "should wrap around")
	m = m.CycleFocus(-1)
	assert.Equal(t, FieldNote, m.Focus(), "should wrap backward")
}

func TestEntryRequiresName(t *testing.T) {
	t.Parallel()
	m := New(ocid, "abc")
	_, ok := m.Entry()
	assert.False(t, ok, "empty name must be rejected")
	m.name.SetValue("   ")
	_, ok = m.Entry()
	assert.False(t, ok, "a name of spaces must be rejected")

	m.name.SetValue(" acme ")
	m.note.SetValue("hi")
	entry, ok := m.Entry()
	require.True(t, ok)
	assert.Equal(t, ocid, entry.ID)
	require.NotNil(t, entry.Name)
	assert.Equal(t, "acme", *entry.Name)
	require.NotNil(t, entry.IsInternal)
	assert.True(t, *entry.IsInternal)
	require.NotNil(t, entry.Note)
	assert.Equal(t, "hi", *entry.Note)

	m.note.SetValue("")
	entry, _ = m.ToggleInternal().Entry()
	assert.Nil(t, entry.Note, "an empty note is left out")
	assert.False(t, *entry.IsInternal)
}

func TestUpdate_Navigation(t *testing.T) {
	t.Parallel()
	m := New(ocid, "abc")
	m, res := press(t, m, "tab")
	assert.Equal(t, None, res.Outcome)
	assert.Equal(t, FieldInternal, m.Focus())
	m, _ = press(t, m, "down")
	assert.Equal(t, FieldNote, m.Focus())
	m, _ = press(t, m, "up", "shift+tab")
	assert.Equal(t, FieldName, m.Focus())
	m, _ = press(t, m, "enter")
	assert.Equal(t, FieldInternal, m.Focus(), "enter moves on before the last field")
}

func TestUpdate_ToggleOnlyOnInternal(t *testing.T) {
	t.Parallel()
	m := New(ocid, "abc")
	m, _ = press(t, m, "space")
	assert.True(t, m.Internal(), "space on Name types, not toggles")
	assert.Equal(t, " ", m.Name())

	m, _ = press(t, New(ocid, "abc"), "tab", "space")
	assert.False(t, m.Internal())
	m, _ = press(t, m, "left")
	assert.True(t, m.Internal())
	m, _ = press(t, m, "x")
	assert.Empty(t, m.Name(), "Internal takes no text")
}

func TestUpdate_TypesIntoFocusedField(t *testing.T) {
	t.Parallel()
	m, _ := press(t, New(ocid, "abc"), "a", "c")
	assert.Equal(t, "ac", m.Name())
	m, _ = press(t, m, "tab", "tab", "n")
	assert.Equal(t, "n", m.Note())
	assert.Equal(t, "ac", m.Name())
}

func TestUpdate_Outcomes(t *testing.T) {
	t.Parallel()
	for k, want := range map[string]Outcome{"esc": Cancel, "ctrl+c": Quit, "ctrl+o": OpenPortal} {
		_, res := press(t, New(ocid, "abc"), k)
		assert.Equal(t, want, res.Outcome, k)
	}
}

func TestUpdate_SaveValidates(t *testing.T) {
	t.Parallel()
	m, res := press(t, New(ocid, "abc"), "tab", "ctrl+s")
	assert.Equal(t, None, res.Outcome, "no name, no save")
	assert.Equal(t, RequiredErr, m.NameErr())
	assert.Equal(t, FieldName, m.Focus())
	assert.Contains(t, ansi.Strip(m.View(80, styles)), RequiredErr)

	m, _ = press(t, m, "a")
	assert.Empty(t, m.NameErr(), "typing clears the mark")

	m, res = press(t, m, "ctrl+s")
	require.Equal(t, Save, res.Outcome, "ctrl+s saves from any field")
	assert.Equal(t, "a", *res.Entry.Name)

	m, res = press(t, m, "enter", "enter")
	assert.Equal(t, None, res.Outcome)
	require.Equal(t, FieldNote, m.Focus())
	_, res = press(t, m, "enter")
	assert.Equal(t, Save, res.Outcome, "enter on the last field saves")
}

func TestUpdate_PasteAndOtherMsgs(t *testing.T) {
	t.Parallel()
	m, _, _ := New(ocid, "abc").Update(tea.PasteMsg{Content: "acme"}, 80)
	assert.Equal(t, "acme", m.Name())
	m = m.FocusOn(FieldNote)
	m, _, _ = m.Update(tea.PasteMsg{Content: "shared"}, 80)
	assert.Equal(t, "shared", m.Note())
	assert.Equal(t, "acme", m.Name())

	m2, res, cmd := m.Update(tea.WindowSizeMsg{}, 80)
	assert.Equal(t, None, res.Outcome)
	assert.Nil(t, cmd)
	assert.Equal(t, m.Name(), m2.Name())
}

func TestIsToggleKey(t *testing.T) {
	t.Parallel()
	for _, k := range []string{" ", "space", "left", "right"} {
		assert.True(t, IsToggleKey(k), k)
	}
	for _, k := range []string{"tab", "x", "enter"} {
		assert.False(t, IsToggleKey(k), k)
	}
}

func TestView(t *testing.T) {
	t.Parallel()
	assert.Empty(t, Model{}.View(80, styles))

	out := ansi.Strip(New(ocid, "abc").View(200, styles))
	assert.Contains(t, out, "Set tenant info")
	assert.Contains(t, out, "abc")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "(•) internal")
	assert.Contains(t, out, "( ) external")
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), MaxWidth, "capped at MaxWidth")
	}

	out = ansi.Strip(New(ocid, "abc").ToggleInternal().View(MaxWidth, styles))
	assert.Contains(t, out, "( ) internal")
	assert.Contains(t, out, "(•) external")
}
