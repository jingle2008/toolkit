// Package tenantform is the TUI's tenant-metadata entry form.
//
// The form attaches a friendly name, an internal/external flag and a
// note to a tenancy that has no metadata name yet. Name is required;
// Internal defaults to true. tab/shift+tab and ↓/↑ cycle the fields,
// enter moves to the next one and saves on the last, ctrl+s saves from
// any field, space/←/→ flip Internal, esc cancels, and the portal key
// asks for the tenancy's DevOps portal page. Saving without a name marks
// the Name field "required" and focuses it, without saving.
//
// Model is a value type in the Bubble Tea nested-model style: Update
// returns the updated Model, which the parent stores back, and a Result
// that says what the keypress decided. The Result is returned
// synchronously, not as a message, so the parent starts the save (or
// closes the form, opens the portal, quits) within the same keypress.
// Writing the entry, the portal URL and what a save does to the dataset
// are the parent's.
package tenantform

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/pkg/models"
)

// MaxWidth caps the form's outer width.
const MaxWidth = 72

// labelWidth is the width of the field-label column.
const labelWidth = 10

// RequiredErr is the mark beside Name after a save without one.
const RequiredErr = "required"

// Field is a focusable form field.
type Field int

// The fields, in focus order.
const (
	FieldName Field = iota
	FieldInternal
	FieldNote
	fieldCount
)

// Outcome is what a keypress decided.
type Outcome int

// The outcomes: None leaves the form open as it is.
const (
	None Outcome = iota
	Save
	Cancel
	OpenPortal
	Quit
)

// Result is what Update decided. On Save the parent persists Entry, the
// validated metadata (its Name trimmed); the form stays open until the
// save lands. On Cancel it closes the form, on OpenPortal it opens the
// tenancy's portal page (OCID), and on Quit it quits.
type Result struct {
	Outcome Outcome
	Entry   models.TenantMetadata
}

// Model is an open form; the zero Model is none.
type Model struct {
	open     bool
	ocid     string // full tenancy OCID — the metadata entry key
	tenantID string // raw tenancy id, shown for context
	name     textinput.Model
	note     textinput.Model
	internal bool
	focus    Field
	nameErr  string
}

// New opens the form on the tenancy ocid, shown as tenantID, with Name
// focused and Internal on (the default for a discovered tenant).
func New(ocid, tenantID string) Model {
	name := textinput.New()
	name.CharLimit = 128
	name.Prompt = ""
	name.Placeholder = "required"
	name.Focus()
	note := textinput.New()
	note.CharLimit = 256
	note.Prompt = ""
	note.Placeholder = "optional"
	return Model{
		open:     true,
		ocid:     ocid,
		tenantID: tenantID,
		name:     name,
		note:     note,
		internal: true,
		focus:    FieldName,
	}
}

// Visible reports whether the form is open.
func (m Model) Visible() bool { return m.open }

// OCID is the tenancy OCID the form edits.
func (m Model) OCID() string { return m.ocid }

// TenantID is the raw tenancy id the form shows.
func (m Model) TenantID() string { return m.tenantID }

// Focus is the focused field.
func (m Model) Focus() Field { return m.focus }

// Name is the Name field as typed.
func (m Model) Name() string { return m.name.Value() }

// Note is the Note field as typed.
func (m Model) Note() string { return m.note.Value() }

// Internal is the Internal flag.
func (m Model) Internal() bool { return m.internal }

// NameErr is the mark beside Name, "" when none.
func (m Model) NameErr() string { return m.nameErr }

// ToggleInternal flips the Internal flag.
func (m Model) ToggleInternal() Model {
	m.internal = !m.internal
	return m
}

// CycleFocus moves the focus by dir (+1/-1), wrapping around.
func (m Model) CycleFocus(dir int) Model {
	return m.FocusOn(Field((int(m.focus) + dir + int(fieldCount)) % int(fieldCount)))
}

// FocusOn moves the focus to field f, focusing its text input if it has
// one.
func (m Model) FocusOn(f Field) Model {
	m.focus = f
	if f == FieldName {
		m.name.Focus()
	} else {
		m.name.Blur()
	}
	if f == FieldNote {
		m.note.Focus()
	} else {
		m.note.Blur()
	}
	return m
}

// Entry builds the TenantMetadata; ok is false when Name is empty or
// only whitespace. The name is trimmed.
func (m Model) Entry() (models.TenantMetadata, bool) {
	name := strings.TrimSpace(m.name.Value())
	if name == "" {
		return models.TenantMetadata{}, false
	}
	internal := m.internal
	entry := models.TenantMetadata{
		ID:         m.ocid,
		Name:       &name,
		IsInternal: &internal,
	}
	if note := m.note.Value(); note != "" {
		entry.Note = &note
	}
	return entry, true
}

// Update routes a key press or a paste while the form is open, the form
// drawn at most maxWidth columns wide (which sizes the text fields, so
// long values scroll). A paste feeds the focused text field; other
// messages are ignored.
//
//nolint:cyclop // form key router; the per-key switch is inherent and splitting it further would obscure the routing surface.
func (m Model) Update(msg tea.Msg, maxWidth int) (Model, Result, tea.Cmd) {
	m = m.sized(maxWidth)
	if _, ok := msg.(tea.PasteMsg); ok {
		cmd := m.updateFocusedField(msg)
		return m, Result{}, cmd
	}
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, Result{}, nil
	}
	k := km.String()
	switch {
	case k == "ctrl+c":
		return m, Result{Outcome: Quit}, nil
	case key.Matches(km, keys.Back):
		return m, Result{Outcome: Cancel}, nil
	case k == "tab", k == "down":
		return m.CycleFocus(1), Result{}, nil
	case k == "shift+tab", k == "up":
		return m.CycleFocus(-1), Result{}, nil
	case key.Matches(km, keys.OpenPortal):
		return m, Result{Outcome: OpenPortal}, nil
	case k == "ctrl+s" || (key.Matches(km, keys.Confirm) && m.focus == fieldCount-1):
		entry, valid := m.Entry()
		if !valid {
			m.nameErr = RequiredErr
			return m.FocusOn(FieldName), Result{}, nil
		}
		return m, Result{Outcome: Save, Entry: entry}, nil
	case key.Matches(km, keys.Confirm):
		return m.CycleFocus(1), Result{}, nil
	case m.focus == FieldInternal && IsToggleKey(k):
		return m.ToggleInternal(), Result{}, nil
	}
	cmd := m.updateFocusedField(msg)
	return m, Result{}, cmd
}

// updateFocusedField feeds msg (a key or a paste) to the focused text
// field; the Internal toggle takes no text. Text in Name clears its
// "required" mark.
func (m *Model) updateFocusedField(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.focus {
	case FieldName:
		m.name, cmd = m.name.Update(msg)
		if m.name.Value() != "" {
			m.nameErr = ""
		}
	case FieldNote:
		m.note, cmd = m.note.Update(msg)
	case FieldInternal, fieldCount:
	}
	return cmd
}

// IsToggleKey reports whether k flips the Internal field. Space is " " in
// bubbletea v1 and "space" in v2; both are accepted.
func IsToggleKey(k string) bool {
	switch k {
	case " ", "space", "left", "right":
		return true
	}
	return false
}

// width is the form's outer width for maxWidth.
func width(maxWidth int) int { return min(maxWidth, MaxWidth) }

// sized fits the text fields to the form drawn at most maxWidth wide.
func (m Model) sized(maxWidth int) Model {
	field := max(width(maxWidth)-4-2-labelWidth, 1) // border+padding, marker, label
	m.name.SetWidth(field - len("  "+RequiredErr))
	m.note.SetWidth(field)
	return m
}

// Styles are the colors the form is drawn in, taken from the parent's
// theme.
type Styles struct {
	Accent color.Color // border, focus marker, selected radio
	Muted  color.Color // labels, portal hint, key hints
	Danger color.Color // the "required" mark
}

// View draws the form at most maxWidth columns wide (and at most
// MaxWidth): the tenancy, a hint for the portal key, then the Name,
// Internal and Note fields with a marker on the focused one. The zero
// Model draws nothing.
func (m Model) View(maxWidth int, st Styles) string {
	if !m.open {
		return ""
	}
	m = m.sized(maxWidth)
	marker := func(f Field) string {
		if m.focus == f {
			return lipgloss.NewStyle().Foreground(st.Accent).Render("▌ ")
		}
		return "  "
	}
	label := lipgloss.NewStyle().Foreground(st.Muted).Width(labelWidth)
	radio := func(on bool, text string) string {
		if on {
			return lipgloss.NewStyle().Foreground(st.Accent).Render("(•)") + " " + text
		}
		return "( ) " + text
	}
	name := m.name.View()
	if m.nameErr != "" {
		name += "  " + lipgloss.NewStyle().Foreground(st.Danger).Render(m.nameErr)
	}
	lines := []string{
		"  " + label.Render("Tenancy") + lipgloss.NewStyle().Bold(true).Render(m.tenantID),
		"  " + label.Render("") + lipgloss.NewStyle().Foreground(st.Muted).
			Render(keys.OpenPortal.Help().Key+" opens its page in the DevOps portal"),
		"",
		marker(FieldName) + label.Render("Name") + name,
		marker(FieldInternal) + label.Render("Internal") +
			radio(m.internal, "internal") + "  " + radio(!m.internal, "external"),
		marker(FieldNote) + label.Render("Note") + m.note.View(),
	}
	hints := "tab/enter next · space toggle · ^s save · esc cancel"
	return view.Modal("Set tenant info", strings.Join(lines, "\n"), hints, st.Accent, st.Muted, width(maxWidth))
}
