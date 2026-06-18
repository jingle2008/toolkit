// Package tui — tenant-metadata entry form (EditTenantView).
//
// Lets the user attach a friendly name / internal flag / note to an
// UNRESOLVED DedicatedAICluster or ImportedModel row, persist it to the
// metadata file via loader.TenantMetadataWriter, and auto-refresh so
// the row resolves.
package tui

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// portalBaseURL is the OCI console metadata-detail page; the tenancy
// OCID is appended as a path segment and the realm as a query param,
// e.g. .../detail/metadata/ocid1.tenancy.oc1..aaaa?realm=oc1
const portalBaseURL = "https://devops.oci.oraclecorp.com/account/admin/detail/metadata/"

// portalURL builds the console portal URL for a tenancy OCID + realm.
func portalURL(ocid, realm string) string {
	return fmt.Sprintf("%s%s?realm=%s", portalBaseURL, ocid, url.QueryEscape(realm))
}

// portalOpenErrMsg reports a failure to launch the browser.
type portalOpenErrMsg struct{ err error }

// editTarget identifies the tenant a row points at and whether it can
// be edited (unresolved + has a real tenancy id).
type editTarget struct {
	ocid     string // full tenancy OCID — the metadata entry key
	tenantID string // raw TenantID suffix, for display context
}

// tenantEditTarget inspects a selected item and returns an editTarget
// when it is an unresolved tenant-owned row (DAC or ImportedModel with
// Owner == nil and a real, non-orphan TenantID). ok is false otherwise.
func tenantEditTarget(item any, realm string) (editTarget, bool) {
	var (
		ocid, tenantID string
		resolved       bool
	)
	switch v := item.(type) {
	case *models.DedicatedAICluster:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	case *models.ImportedModel:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	default:
		return editTarget{}, false
	}
	if resolved || tenantID == "" || tenantID == "UNKNOWN_TENANCY" {
		return editTarget{}, false
	}
	return editTarget{ocid: ocid, tenantID: tenantID}, true
}

// Focus indices for the form fields.
const (
	focusName = iota
	focusInternal
	focusNote
	focusCount
)

type editTenantForm struct {
	ocid       string
	tenantID   string
	name       textinput.Model
	note       textinput.Model
	isInternal bool
	focus      int
}

// tenantSavedMsg / tenantSaveErrMsg report the async upsert result.
// tenantSavedMsg carries the persisted entry so the reducer can apply
// the same resolution in memory (no reload).
type (
	tenantSavedMsg struct {
		path  string
		entry models.TenantMetadata
	}
	tenantSaveErrMsg struct{ err error }
)

func newEditTenantForm(t editTarget) *editTenantForm {
	name := textinput.New()
	name.CharLimit = 128
	name.Prompt = ""
	name.Focus()
	note := textinput.New()
	note.CharLimit = 256
	note.Prompt = ""
	return &editTenantForm{
		ocid:       t.ocid,
		tenantID:   t.tenantID,
		name:       name,
		note:       note,
		isInternal: true, // matches getTenants' discovered-tenant default
		focus:      focusName,
	}
}

func (f *editTenantForm) toggleInternal() { f.isInternal = !f.isInternal }

// toEntry builds the TenantMetadata; ok is false when Name is empty.
func (f *editTenantForm) toEntry() (models.TenantMetadata, bool) {
	name := f.name.Value()
	if name == "" {
		return models.TenantMetadata{}, false
	}
	entry := models.TenantMetadata{
		ID:         f.ocid,
		Name:       &name,
		IsInternal: &f.isInternal,
	}
	if note := f.note.Value(); note != "" {
		entry.Note = &note
	}
	return entry, true
}

// cycleFocus moves focus by dir (+1/-1) and updates textinput focus.
func (f *editTenantForm) cycleFocus(dir int) {
	f.focus = (f.focus + dir + focusCount) % focusCount
	if f.focus == focusName {
		f.name.Focus()
	} else {
		f.name.Blur()
	}
	if f.focus == focusNote {
		f.note.Focus()
	} else {
		f.note.Blur()
	}
}

// openTenantForm gates on the selected item and, when editable, opens
// the form. Returns a cmd (toast on rejection, blink on open).
func (m *Model) openTenantForm(item any) tea.Cmd {
	tgt, ok := tenantEditTarget(item, m.environment.Realm)
	if !ok {
		return m.showToast("tenant already resolved or has no tenancy id", toastWarn)
	}
	m.editTenant = newEditTenantForm(tgt)
	m.lastViewMode = m.viewMode
	m.viewMode = common.EditTenantView
	return textinput.Blink
}

// enterEditTenantView is the key-handler entry point.
func (m *Model) enterEditTenantView() tea.Cmd {
	return m.openTenantForm(m.selectedItem())
}

// updateEditTenantView handles key events while the form is open. The
// async save-result messages (tenantSavedMsg / tenantSaveErrMsg) are
// intercepted at the top of Update so they fire regardless of the
// active view, and therefore never reach here.
func (m *Model) updateEditTenantView(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || m.editTenant == nil {
		return m, nil
	}
	return m.handleEditTenantKey(keyMsg)
}

// handleTenantSavedMsg finalizes a successful tenant-metadata save. It
// runs regardless of the current view: the user may have dismissed the
// form (esc) before the async write landed, so it keys off editTenant
// state, not viewMode.
func (m *Model) handleTenantSavedMsg(msg tenantSavedMsg) tea.Cmd {
	if m.editTenant != nil {
		m.editTenant = nil
	}
	if m.viewMode == common.EditTenantView {
		m.viewMode = common.ListView
	}
	m.applyTenantSave(msg.entry)
	return m.showToast(fmt.Sprintf("saved tenant metadata to %s", msg.path), toastInfo)
}

// handleTenantSaveErrMsg surfaces a failed save. The form (if still
// open) is left intact so the user's input isn't lost.
func (m *Model) handleTenantSaveErrMsg(msg tenantSaveErrMsg) tea.Cmd {
	return m.showToast(fmt.Sprintf("save failed: %v", msg.err), toastError)
}

// handleEditTenantKey routes a key event while the form is open: nav
// keys cycle focus, Confirm validates+saves, Back cancels, and the
// remaining keys feed the focused text field.
//
//nolint:cyclop // form key router; the per-key switch is inherent and splitting it further would obscure the routing surface.
func (m *Model) handleEditTenantKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.editTenant
	switch {
	case keyMsg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case key.Matches(keyMsg, keys.Back):
		m.editTenant = nil
		m.viewMode = common.ListView
		return m, nil
	case keyMsg.Type == tea.KeyTab, keyMsg.Type == tea.KeyDown:
		f.cycleFocus(1)
		return m, nil
	case keyMsg.Type == tea.KeyShiftTab, keyMsg.Type == tea.KeyUp:
		f.cycleFocus(-1)
		return m, nil
	case key.Matches(keyMsg, keys.OpenPortal):
		return m, m.openPortalCmd()
	case key.Matches(keyMsg, keys.Confirm):
		entry, valid := f.toEntry()
		if !valid {
			return m, m.showToast("name is required", toastWarn)
		}
		return m, m.saveTenantMetadataCmd(entry)
	case f.focus == focusInternal &&
		(keyMsg.Type == tea.KeySpace || keyMsg.Type == tea.KeyLeft || keyMsg.Type == tea.KeyRight):
		f.toggleInternal()
		return m, nil
	}

	// Route remaining keys to the focused text field.
	var cmd tea.Cmd
	switch f.focus {
	case focusName:
		f.name, cmd = f.name.Update(keyMsg)
	case focusNote:
		f.note, cmd = f.note.Update(keyMsg)
	}
	return m, cmd
}

// openPortalCmd launches the OCI console portal for the form's tenancy
// OCID in the user's browser, off the UI goroutine. The URL is built
// before the closure so it doesn't read m concurrently.
func (m *Model) openPortalCmd() tea.Cmd {
	if m.editTenant == nil {
		return nil
	}
	target := portalURL(m.editTenant.ocid, m.environment.Realm)
	return func() tea.Msg {
		if err := actions.OpenURL(target); err != nil {
			return portalOpenErrMsg{err: err}
		}
		return nil
	}
}

// saveTenantMetadataCmd persists the entry via the optional loader
// writer interface, off the UI goroutine.
func (m *Model) saveTenantMetadataCmd(entry models.TenantMetadata) tea.Cmd {
	writer, ok := m.loader.(loader.TenantMetadataWriter)
	path := m.metadataPath()
	return func() tea.Msg {
		if !ok {
			return tenantSaveErrMsg{err: errors.New("loader does not support writing metadata")}
		}
		if err := writer.UpsertTenantMetadata(entry); err != nil {
			return tenantSaveErrMsg{err: err}
		}
		return tenantSavedMsg{path: path, entry: entry}
	}
}

// applyTenantSave reflects a just-saved tenant entry in the dataset
// entirely in memory — no reload. Because the form only edits UNRESOLVED
// rows, the entry is always a standalone tenant (getTenants' second
// pass): its tenancy suffix matched no existing Tenant, which also means
// it has no tenancy-override entries. So we add the one Tenant directly
// and re-resolve the tenant-owned maps against it; the override maps are
// provably unaffected and left alone.
//
// Both the DAC and ImportedModel maps are re-keyed (not just the current
// category) so a tenant owning resources in both resolves everywhere at
// once. Each item retains its raw TenantID after the earlier name-keying,
// so the suffix-keyed raw map is reconstructed from what's already loaded.
//
// If the form is ever extended to edit RESOLVED tenants, this assumption
// breaks (a rename could affect override-map display) and a fuller
// rebuild would be needed.
func (m *Model) applyTenantSave(entry models.TenantMetadata) {
	ds := m.dataset
	if ds == nil {
		return
	}
	ds.Tenants = upsertTenantByID(ds.Tenants, tenantFromEntry(entry))
	if ds.DedicatedAIClusterMap != nil {
		ds.SetDedicatedAIClusterMap(rawByTenantID(ds.DedicatedAIClusterMap,
			func(d models.DedicatedAICluster) string { return d.TenantID }))
	}
	if ds.ImportedModelMap != nil {
		ds.SetImportedModelMap(rawByTenantID(ds.ImportedModelMap,
			func(i models.ImportedModel) string { return i.TenantID }))
	}
	m.refreshDisplay()
}

// tenantFromEntry builds a Tenant from a saved metadata entry.
func tenantFromEntry(e models.TenantMetadata) models.Tenant {
	t := models.Tenant{IDs: []string{e.ID}}
	if e.Name != nil {
		t.Name = *e.Name
	}
	if e.IsInternal != nil {
		t.IsInternal = *e.IsInternal
	}
	if e.Note != nil {
		t.Note = *e.Note
	}
	return t
}

// upsertTenantByID replaces the tenant whose IDs contain t's ID, else
// appends t. (Replacement only matters defensively; the unresolved-only
// gate means the form never re-edits an already-present tenant.)
func upsertTenantByID(tenants []models.Tenant, t models.Tenant) []models.Tenant {
	id := t.IDs[0]
	for i := range tenants {
		if slices.Contains(tenants[i].IDs, id) {
			tenants[i] = t
			return tenants
		}
	}
	return append(tenants, t)
}

// rawByTenantID rebuilds the raw (TenantID-keyed) map from an
// already-loaded, name-keyed tenant-owned map, so SetXxxMap can
// re-resolve Owner against the current Tenants without a re-fetch.
func rawByTenantID[T any](mm map[string][]T, tenantID func(T) string) map[string][]T {
	raw := make(map[string][]T, len(mm))
	for _, items := range mm {
		for _, it := range items {
			k := tenantID(it)
			raw[k] = append(raw[k], it)
		}
	}
	return raw
}

// editTenantView renders the form overlay.
func (m *Model) editTenantView() string {
	f := m.editTenant
	if f == nil {
		return ""
	}
	marker := func(i int) string {
		if f.focus == i {
			return "> "
		}
		return "  "
	}
	internal := "external"
	if f.isInternal {
		internal = "internal"
	}
	lines := []string{
		fmt.Sprintf("Set tenant info for %s", f.tenantID),
		"",
		marker(focusName) + "Name:     " + f.name.View(),
		marker(focusInternal) + "Internal: " + internal + "  (space/left/right to toggle)",
		marker(focusNote) + "Note:     " + f.note.View(),
		"",
		m.help.ShortHelpView([]key.Binding{keys.Confirm, keys.OpenPortal, keys.Back}),
	}
	return m.helpBorder.Width(m.viewWidth * 3 / 5).Render(strings.Join(lines, "\n"))
}

// metadataPath returns the configured metadata file path for display,
// best-effort via an optional getter on the loader; a placeholder when
// unavailable.
func (m *Model) metadataPath() string {
	if p, ok := m.loader.(interface{ MetadataPath() string }); ok {
		return p.MetadataPath()
	}
	return "metadata file"
}
