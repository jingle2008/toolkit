// Package tui — tenant-metadata entry form (EditTenantView).
//
// Lets the user attach a friendly name / internal flag / note to an
// UNRESOLVED DedicatedAICluster or ImportedModel row, persist it to the
// metadata file via loader.TenantMetadataWriter, and auto-refresh so
// the row resolves. The form itself — fields, focus, validation, key
// routing, rendering — is tenantform's; this file gates and opens it,
// runs its outcomes, and applies a save to the dataset.
package tui

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/tenantform"
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

// tenantSavedMsg / tenantSaveErrMsg report the async upsert result.
// tenantSavedMsg carries the persisted entry so the reducer can apply
// the same resolution in memory (no reload).
type (
	tenantSavedMsg struct {
		path  string
		entry models.TenantMetadata
		realm string // the realm the tenant was saved in
	}
	tenantSaveErrMsg struct{ err error }
)

// openTenantForm gates on the selected item and, when editable, opens
// the form. Returns a cmd (toast on rejection, blink on open).
func (m *Model) openTenantForm(item any) tea.Cmd {
	tgt, ok := tenantEditTarget(item, m.environment.Realm)
	if !ok {
		return m.showToast("tenant already resolved or has no tenancy id", toastWarn)
	}
	m.editTenant = tenantform.New(tgt.ocid, tgt.tenantID)
	m.lastViewMode = m.viewMode
	m.viewMode = common.EditTenantView
	return textinput.Blink
}

// enterEditTenantView is the key-handler entry point.
func (m *Model) enterEditTenantView() tea.Cmd {
	return m.openTenantForm(m.selectedItem())
}

// updateEditTenantView resolves key events and pastes while the form is
// open: the form decides (tenantform.Model.Update), and the parent runs
// the outcome — a save, a cancel, the portal, a quit. The async
// save-result messages (tenantSavedMsg / tenantSaveErrMsg) are
// intercepted at the top of Update so they fire regardless of the active
// view, and therefore never reach here.
func (m *Model) updateEditTenantView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.editTenant.Visible() {
		return m, nil
	}
	var res tenantform.Result
	var cmd tea.Cmd
	m.editTenant, res, cmd = m.editTenant.Update(msg, m.modalMaxWidth())
	switch res.Outcome {
	case tenantform.Quit:
		return m, m.quit()
	case tenantform.Cancel:
		m.editTenant = tenantform.Model{}
		m.viewMode = common.ListView
		return m, nil
	case tenantform.OpenPortal:
		return m, m.openPortalCmd()
	case tenantform.Save:
		return m, m.saveTenantMetadataCmd(res.Entry)
	case tenantform.None:
	}
	return m, cmd
}

// handleTenantSavedMsg finalizes a successful tenant-metadata save. It
// runs regardless of the current view: the user may have dismissed the
// form (esc) before the async write landed, so it keys off editTenant
// state, not viewMode.
func (m *Model) handleTenantSavedMsg(msg tenantSavedMsg) tea.Cmd {
	m.editTenant = tenantform.Model{}
	if m.viewMode == common.EditTenantView {
		m.viewMode = common.ListView
	}
	// The save is a tracked action and outlives an environment switch;
	// tenants are per realm, so one saved in another realm is left to
	// load from the file when that realm is shown again.
	if msg.realm == m.environment.Realm {
		m.applyTenantSave(msg.entry)
	}
	return m.showToast(fmt.Sprintf("saved tenant metadata to %s", msg.path), toastInfo)
}

// handleTenantSaveErrMsg surfaces a failed save. The form (if still
// open) is left intact so the user's input isn't lost.
func (m *Model) handleTenantSaveErrMsg(msg tenantSaveErrMsg) tea.Cmd {
	return m.showErrorWithNext(fmt.Sprintf("save failed: %v", msg.err), "^s retry") // the form keeps the input
}

// openPortalCmd launches the OCI console portal for the form's tenancy
// OCID in the user's browser, off the UI goroutine. The URL is built
// before the closure so it doesn't read m concurrently.
func (m *Model) openPortalCmd() tea.Cmd {
	if !m.editTenant.Visible() {
		return nil
	}
	target := portalURL(m.editTenant.OCID(), m.environment.Realm)
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
	path, realm, logger := m.metadataPath(), m.environment.Realm, m.logger
	return m.trackAction("save tenant "+entry.ID, "", nil, func(func(string)) tea.Msg {
		if !ok {
			return tenantSaveErrMsg{err: errors.New("loader does not support writing metadata")}
		}
		if err := audited(logger, mutation.SetTenant, entry.ID, func() error {
			return writer.UpsertTenantMetadata(entry)
		}); err != nil {
			return tenantSaveErrMsg{err: err}
		}
		return tenantSavedMsg{path: path, entry: entry, realm: realm}
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
	if m.dataset == nil {
		return
	}
	m.editDataset(func(ds *models.Dataset) {
		ds.Tenants = upsertTenantByID(ds.Tenants, tenantFromEntry(entry))
		resolveTenantOwners(ds)
		renameOverrideTenants(ds)
	})
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
// keyed by node/namespace, so their owners are re-resolved in place, on
// copies: a tea.Cmd may still be reading the old ones.
func resolveTenantOwners(ds *models.Dataset) {
	ds.GPUWorkloadMap = cloneGroups(ds.GPUWorkloadMap)
	ds.InferenceServiceMap = cloneGroups(ds.InferenceServiceMap)
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
	if i := slices.IndexFunc(tenants, func(e models.Tenant) bool { return e.ID == t.ID }); i >= 0 {
		tenants = slices.Clone(tenants) // a tea.Cmd may hold the old element
		tenants[i] = t
		return tenants
	}
	return append(slices.Clip(tenants), t)
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

// metadataPath returns the configured metadata file path for display,
// best-effort via an optional getter on the loader; a placeholder when
// unavailable.
func (m *Model) metadataPath() string {
	if p, ok := m.loader.(interface{ MetadataPath() string }); ok {
		return p.MetadataPath()
	}
	return "metadata file"
}

// editTenantView renders the form overlay in the theme, sized to the
// screen; "" when no form is open.
func (m *Model) editTenantView() string {
	p := m.theme.Palette
	return m.editTenant.View(m.modalMaxWidth(),
		tenantform.Styles{Accent: p.Accent, Muted: p.Muted, Danger: p.Danger})
}
