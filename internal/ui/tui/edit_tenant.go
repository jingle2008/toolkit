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

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
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

// tenantRekeyMsg carries the raw (suffix-keyed) tenant-owned map to be
// re-resolved against freshly-loaded Tenants in memory, avoiding a
// cluster re-fetch after a metadata save. Exactly one of dac/imported is
// populated, per category.
type tenantRekeyMsg struct {
	gen      int
	category domain.Category
	dac      map[string][]models.DedicatedAICluster
	imported map[string][]models.ImportedModel
}

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
type (
	tenantSavedMsg   struct{ path string }
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
	return tea.Batch(
		m.showToast(fmt.Sprintf("saved tenant metadata to %s", msg.path), toastInfo),
		m.reloadAfterTenantSave(),
	)
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
		return tenantSavedMsg{path: path}
	}
}

// reloadAfterTenantSave reloads only the tenancy-override group (LOCAL
// repo read — rebuilds Tenants from the new metadata) and then re-keys
// the current category's map IN MEMORY against those fresh Tenants,
// avoiding a cluster re-fetch. Each DAC/ImportedModel item still carries
// its raw TenantID after the earlier name-keying, so the suffix-keyed
// raw map can be reconstructed from what's already loaded.
//
// The current map is deliberately NOT nil'd: it keeps showing the prior
// (correct-except-the-just-saved-row) data until the re-key lands a beat
// later, so there's no empty-table flash. The tenancyOverridesLoaded
// handler overwrites Tenants + the three override maps wholesale, so
// they need no nil'ing either.
//
// NOTE: only the CURRENT category is re-keyed. The sibling tenant-owned
// map (DAC vs ImportedModel) keeps stale Owner pointers into the old
// Tenants slice until it is itself reloaded.
func (m *Model) reloadAfterTenantSave() tea.Cmd {
	ds := m.dataset
	if ds == nil {
		return nil
	}
	// Reconstruct the raw (suffix-keyed) map from the in-memory items so
	// the re-key can re-resolve Owner without a cluster round-trip.
	rekey := tenantRekeyMsg{category: m.category}
	switch m.category {
	case domain.DedicatedAICluster:
		raw := make(map[string][]models.DedicatedAICluster, len(ds.DedicatedAIClusterMap))
		for _, items := range ds.DedicatedAIClusterMap {
			for _, it := range items {
				raw[it.TenantID] = append(raw[it.TenantID], it)
			}
		}
		rekey.dac = raw
	case domain.ImportedModel:
		raw := make(map[string][]models.ImportedModel, len(ds.ImportedModelMap))
		for _, items := range ds.ImportedModelMap {
			for _, it := range items {
				raw[it.TenantID] = append(raw[it.TenantID], it)
			}
		}
		rekey.imported = raw
	default:
		return nil
	}

	m.newLoadContext()
	gen := m.bumpGen()
	rekey.gen = gen
	grp := loadTenancyOverrideGroupCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	// Sequence guarantees the group's loaded-msg is enqueued (and so
	// processed, rebuilding Tenants) before the rekey msg — see Update's
	// FIFO handling. Only the group load is a task; the re-key is instant.
	rekeyCmd := func() tea.Msg { return rekey }
	return tea.Sequence(m.beginTask(), grp, rekeyCmd)
}

// handleTenantRekeyMsg re-resolves the current category's tenant-owned
// map against the freshly-loaded Tenants, in memory. It runs after the
// tenancy-override group load (which sets ds.Tenants); the gen guard
// drops it if the user navigated away meanwhile. The re-key is not a
// task, so there is no endTask here.
func (m *Model) handleTenantRekeyMsg(msg tenantRekeyMsg) {
	if msg.gen != m.gen || m.dataset == nil {
		return
	}
	switch msg.category {
	case domain.DedicatedAICluster:
		m.dataset.SetDedicatedAIClusterMap(msg.dac)
	case domain.ImportedModel:
		m.dataset.SetImportedModelMap(msg.imported)
	default:
		return
	}
	m.refreshDisplay()
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
