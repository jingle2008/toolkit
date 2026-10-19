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
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/mutation"
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
// when its tenant has no metadata name yet, in any category with a Tenant
// column. A tenant-owned row (DAC, ImportedModel, GPUWorkload,
// InferenceService) qualifies when its tenancy resolves to no tenant
// (Owner == nil) and is real, not an orphan's. A tenancy override always
// has a tenant (one per override OCID); it qualifies while that tenant is
// named by its OCID. ok is false otherwise.
func tenantEditTarget(item any, realm string) (editTarget, bool) {
	ocid, tenantID, resolved, ok := ownedTenant(item, realm)
	if !ok {
		ocid, tenantID, resolved, ok = overrideTenant(item)
	}
	if !ok || resolved || tenantID == "" || tenantID == models.UnknownTenancy {
		return editTarget{}, false
	}
	return editTarget{ocid: ocid, tenantID: tenantID}, true
}

// ownedTenant is a tenant-owned row's tenancy: its OCID, its raw tenancy
// id, and whether it resolves to a tenant. ok is false for other rows.
func ownedTenant(item any, realm string) (ocid, tenantID string, resolved, ok bool) {
	switch v := item.(type) {
	case *models.DedicatedAICluster:
		if v != nil {
			return v.TenancyOCID(realm), v.TenantID, v.Owner != nil, true
		}
	case *models.ImportedModel:
		if v != nil {
			return v.TenancyOCID(realm), v.TenantID, v.Owner != nil, true
		}
	case *models.GPUWorkload:
		if v != nil {
			return v.TenancyOCID(realm), v.TenantID, v.Owner != nil, true
		}
	case *models.InferenceService:
		if v != nil {
			return v.TenancyOCID(realm), v.TenantID, v.Owner != nil, true
		}
	}
	return "", "", false, false
}

// overrideTenant is a tenancy override's tenancy: its OCID (twice: it is
// also the id), and whether its tenant has a metadata name, i.e. isn't
// named by the OCID. ok is false for other rows.
func overrideTenant(item any) (ocid, tenantID string, named, ok bool) {
	var id, name string
	switch v := item.(type) {
	case *models.LimitTenancyOverride:
		if v == nil {
			return "", "", false, false
		}
		id, name = v.TenantID, v.TenantName
	case *models.ConsolePropertyTenancyOverride:
		if v == nil {
			return "", "", false, false
		}
		id, name = v.TenantID, v.TenantName
	case *models.PropertyTenancyOverride:
		if v == nil {
			return "", "", false, false
		}
		id, name = v.TenantID, v.TenantName
	default:
		return "", "", false, false
	}
	return id, id, name != id, true
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
	nameErr    string // shown beside Name after a save without one
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
	name.Placeholder = "required"
	name.Focus()
	note := textinput.New()
	note.CharLimit = 256
	note.Prompt = ""
	note.Placeholder = "optional"
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
	f.focusOn((f.focus + dir + focusCount) % focusCount)
}

// focusOn moves focus to field i, focusing its text input if it has one.
func (f *editTenantForm) focusOn(i int) {
	f.focus = i
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
	keyMsg, ok := msg.(tea.KeyPressMsg)
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
	return m.showErrorWithNext(fmt.Sprintf("save failed: %v", msg.err), "^s retry") // the form keeps the input
}

// handleEditTenantKey routes a key event while the form is open: tab
// and the arrows cycle focus, enter moves to the next field and saves on
// the last, ctrl+s saves from any field, Back cancels, and the remaining
// keys feed the focused text field. Saving without a name marks the Name
// field and focuses it.
//
//nolint:cyclop // form key router; the per-key switch is inherent and splitting it further would obscure the routing surface.
func (m *Model) handleEditTenantKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.editTenant
	k := keyMsg.String()
	switch {
	case k == "ctrl+c":
		return m, m.quit()
	case key.Matches(keyMsg, keys.Back):
		m.editTenant = nil
		m.viewMode = common.ListView
		return m, nil
	case k == "tab", k == "down":
		f.cycleFocus(1)
		return m, nil
	case k == "shift+tab", k == "up":
		f.cycleFocus(-1)
		return m, nil
	case key.Matches(keyMsg, keys.OpenPortal):
		return m, m.openPortalCmd()
	case k == "ctrl+s" || (key.Matches(keyMsg, keys.Confirm) && f.focus == focusCount-1):
		entry, valid := f.toEntry()
		if !valid {
			f.nameErr = "required"
			f.focusOn(focusName)
			return m, nil
		}
		return m, m.saveTenantMetadataCmd(entry)
	case key.Matches(keyMsg, keys.Confirm):
		f.cycleFocus(1)
		return m, nil
	case f.focus == focusInternal && isToggleKey(k):
		f.toggleInternal()
		return m, nil
	}

	// Route remaining keys to the focused text field.
	var cmd tea.Cmd
	switch f.focus {
	case focusName:
		f.name, cmd = f.name.Update(keyMsg)
		if f.name.Value() != "" {
			f.nameErr = ""
		}
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
	return m.trackAction("save tenant "+entry.ID, "", nil, func(func(string)) tea.Msg {
		if !ok {
			return tenantSaveErrMsg{err: errors.New("loader does not support writing metadata")}
		}
		if err := audited(m.logger, mutation.SetTenant, entry.ID, func() error {
			return writer.UpsertTenantMetadata(entry)
		}); err != nil {
			return tenantSaveErrMsg{err: err}
		}
		return tenantSavedMsg{path: path, entry: entry}
	})
}

// applyTenantSave reflects a just-saved tenant entry in the dataset
// entirely in memory — no reload. The form only edits a tenant without a
// metadata name, so the entry either adds a tenant (a tenant-owned row's
// tenancy matched none) or names one a tenancy override's OCID made, which
// renames it from its OCID. upsertTenantByID covers both; the tenant-owned
// maps are re-resolved against the result, and the override maps, keyed
// and stamped by tenant name, are regrouped under the new name.
//
// Every tenant-owned map is re-resolved (see resolveTenantOwners), even
// when its owners don't change: upsertTenantByID may reallocate
// ds.Tenants, which would leave Owner pointers into the old array.
func (m *Model) applyTenantSave(entry models.TenantMetadata) {
	ds := m.dataset
	if ds == nil {
		return
	}
	ds.Tenants = upsertTenantByID(ds.Tenants, tenantFromEntry(entry))
	resolveTenantOwners(ds)
	renameOverrideTenants(ds)
	m.refreshDisplay()
}

// renameOverrideTenants regroups the tenancy-override maps by their
// records' current tenant names, as the loader groups them, so a tenant
// just named moves its overrides from under its OCID.
func renameOverrideTenants(ds *models.Dataset) {
	nameOf := make(map[string]string, len(ds.Tenants))
	for _, t := range ds.Tenants {
		nameOf[t.ID] = t.Name
	}
	ds.LimitTenancyOverrideMap = regroupByTenant(ds.LimitTenancyOverrideMap, nameOf, (*models.LimitTenancyOverride).SetTenantName)
	ds.ConsolePropertyTenancyOverrideMap = regroupByTenant(ds.ConsolePropertyTenancyOverrideMap, nameOf, (*models.ConsolePropertyTenancyOverride).SetTenantName)
	ds.PropertyTenancyOverrideMap = regroupByTenant(ds.PropertyTenancyOverrideMap, nameOf, (*models.PropertyTenancyOverride).SetTenantName)
}

// regroupByTenant re-keys mm by each record's tenant name in nameOf,
// stamping it on the record; a record whose tenant isn't there keeps its
// group. nil stays nil (not loaded).
func regroupByTenant[T models.TenancyOverride](mm map[string][]T, nameOf map[string]string, setName func(*T, string)) map[string][]T {
	if mm == nil {
		return nil
	}
	out := make(map[string][]T, len(mm))
	for group, items := range mm {
		for _, it := range items {
			name, ok := nameOf[it.GetTenantID()]
			if !ok {
				name = group
			}
			setName(&it, name)
			out[name] = append(out[name], it)
		}
	}
	return out
}

// resolveTenantOwners re-resolves every loaded tenant-owned map against
// ds.Tenants. Both the DAC and ImportedModel maps are re-keyed (not just
// the current category) so a tenant owning resources in both resolves
// everywhere at once. Each item retains its raw TenantID after the
// earlier name-keying, so the suffix-keyed raw map is reconstructed from
// what's already loaded. The GPUWorkload and InferenceService maps are
// keyed by node/namespace, so their owners are re-resolved in place.
func resolveTenantOwners(ds *models.Dataset) {
	if ds.DedicatedAIClusterMap != nil {
		ds.SetDedicatedAIClusterMap(rawByTenantID(ds.DedicatedAIClusterMap,
			func(d models.DedicatedAICluster) string { return d.TenantID }))
	}
	if ds.ImportedModelMap != nil {
		ds.SetImportedModelMap(rawByTenantID(ds.ImportedModelMap,
			func(i models.ImportedModel) string { return i.TenantID }))
	}
	if ds.GPUWorkloadMap != nil {
		ds.SetGPUWorkloadMap(ds.GPUWorkloadMap)
	}
	if ds.InferenceServiceMap != nil {
		ds.SetInferenceServiceMap(ds.InferenceServiceMap)
	}
}

// tenantFromEntry builds a Tenant from a saved metadata entry.
func tenantFromEntry(e models.TenantMetadata) models.Tenant {
	t := models.Tenant{ID: e.ID, Name: e.ID}
	if e.Name != nil && *e.Name != "" {
		t.Name = *e.Name
	}
	if e.IsInternal != nil {
		t.IsInternal = *e.IsInternal
	}
	if e.Note != nil {
		t.Note = *e.Note
	}
	t.Status = e.Status
	return t
}

// upsertTenantByID replaces the tenant with t's ID, else appends t. A
// tenancy override's tenant is already present, named by its OCID, and is
// replaced by its newly named self.
func upsertTenantByID(tenants []models.Tenant, t models.Tenant) []models.Tenant {
	for i := range tenants {
		if tenants[i].ID == t.ID {
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
	p := m.theme.Palette
	marker := func(i int) string {
		if f.focus == i {
			return lipgloss.NewStyle().Foreground(p.Accent).Render("▌ ")
		}
		return "  "
	}
	label := lipgloss.NewStyle().Foreground(p.Muted).Width(10)
	radio := func(on bool, text string) string {
		if on {
			return lipgloss.NewStyle().Foreground(p.Accent).Render("(•)") + " " + text
		}
		return "( ) " + text
	}
	width := min(m.modalMaxWidth(), 72)
	field := max(width-4-2-label.GetWidth(), 1) // border+padding, marker, label
	f.name.SetWidth(field - len("  required"))
	f.note.SetWidth(field)
	name := f.name.View()
	if f.nameErr != "" {
		name += "  " + lipgloss.NewStyle().Foreground(p.Danger).Render(f.nameErr)
	}
	lines := []string{
		"  " + label.Render("Tenancy") + lipgloss.NewStyle().Bold(true).Render(f.tenantID),
		"  " + label.Render("") + lipgloss.NewStyle().Foreground(p.Muted).
			Render(keys.OpenPortal.Help().Key+" opens its page in the DevOps portal"),
		"",
		marker(focusName) + label.Render("Name") + name,
		marker(focusInternal) + label.Render("Internal") +
			radio(f.isInternal, "internal") + "  " + radio(!f.isInternal, "external"),
		marker(focusNote) + label.Render("Note") + f.note.View(),
	}
	hints := "tab/enter next · space toggle · ^s save · esc cancel"
	return m.modal("Set tenant info", strings.Join(lines, "\n"), hints, p.Accent, width)
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

// isToggleKey reports whether k flips the Internal field. Space is " " in
// bubbletea v1 and "space" in v2; both are accepted so the swap needs no
// change here.
func isToggleKey(k string) bool {
	switch k {
	case " ", "space", "left", "right":
		return true
	}
	return false
}
