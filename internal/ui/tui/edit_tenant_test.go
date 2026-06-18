package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTenantEditTarget(t *testing.T) {
	t.Parallel()

	realm := "oc1"

	// Unresolved DAC → editable, OCID built from realm + TenantID.
	dac := &models.DedicatedAICluster{Name: "dac1", TenantID: "abc"}
	tgt, ok := tenantEditTarget(dac, realm)
	if !ok {
		t.Fatal("unresolved DAC should be a valid target")
	}
	if tgt.ocid != "ocid1.tenancy.oc1..abc" {
		t.Fatalf("ocid: got %q", tgt.ocid)
	}
	if tgt.tenantID != "abc" {
		t.Fatalf("tenantID: got %q", tgt.tenantID)
	}

	// Resolved DAC (Owner set) → not editable.
	resolved := &models.DedicatedAICluster{Name: "dac2", TenantID: "abc", Owner: &models.Tenant{Name: "acme"}}
	if _, ok := tenantEditTarget(resolved, realm); ok {
		t.Fatal("resolved DAC must not be editable")
	}

	// Orphan (UNKNOWN_TENANCY) → not editable.
	orphan := &models.ImportedModel{TenantID: "UNKNOWN_TENANCY"}
	if _, ok := tenantEditTarget(orphan, realm); ok {
		t.Fatal("orphan tenant must not be editable")
	}

	// Empty TenantID → not editable.
	empty := &models.ImportedModel{TenantID: ""}
	if _, ok := tenantEditTarget(empty, realm); ok {
		t.Fatal("empty TenantID must not be editable")
	}

	// Unresolved ImportedModel → editable.
	im := &models.ImportedModel{TenantID: "xyz"}
	tgt, ok = tenantEditTarget(im, realm)
	if !ok || tgt.ocid != "ocid1.tenancy.oc1..xyz" {
		t.Fatalf("unresolved ImportedModel: ok=%v ocid=%q", ok, tgt.ocid)
	}

	// Unrelated type → not a target.
	if _, ok := tenantEditTarget(&models.GPUPool{}, realm); ok {
		t.Fatal("non tenant-owned type must not be a target")
	}
}

func TestEnterEditTenantView_GatingAndOpen(t *testing.T) {
	t.Parallel()

	m := makeTestModel()
	m.environment.Realm = "oc1"

	// Resolved row → no form, stays in list view.
	m.editTenant = nil
	m.viewMode = common.ListView
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc", Owner: &models.Tenant{Name: "acme"}})
	if m.viewMode != common.ListView || m.editTenant != nil {
		t.Fatal("resolved row must not open the form")
	}

	// Unresolved row → form opens, seeded with internal=true default.
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc"})
	if m.viewMode != common.EditTenantView || m.editTenant == nil {
		t.Fatal("unresolved row should open the form")
	}
	if m.editTenant.ocid != "ocid1.tenancy.oc1..abc" {
		t.Fatalf("form ocid: got %q", m.editTenant.ocid)
	}
	if !m.editTenant.isInternal {
		t.Fatal("internal should default to true")
	}

	// Toggle internal.
	m.editTenant.toggleInternal()
	if m.editTenant.isInternal {
		t.Fatal("toggleInternal should flip to false")
	}
}

func TestPortalURL(t *testing.T) {
	t.Parallel()

	got := portalURL("ocid1.tenancy.oc1..abc", "oc1")
	want := "https://devops.oci.oraclecorp.com/account/admin/detail/metadata/ocid1.tenancy.oc1..abc?realm=oc1"
	if got != want {
		t.Fatalf("portalURL:\n got %q\nwant %q", got, want)
	}
}

func TestEditTenantForm_EntryRequiresName(t *testing.T) {
	t.Parallel()

	f := newEditTenantForm(editTarget{ocid: "ocid1.tenancy.oc1..abc", tenantID: "abc"})
	if _, ok := f.toEntry(); ok {
		t.Fatal("empty name must be rejected")
	}
	f.name.SetValue("acme")
	f.note.SetValue("hi")
	f.isInternal = true
	entry, ok := f.toEntry()
	if !ok {
		t.Fatal("valid form should produce an entry")
	}
	if entry.ID != "ocid1.tenancy.oc1..abc" || entry.Name == nil || *entry.Name != "acme" ||
		entry.IsInternal == nil || !*entry.IsInternal || entry.Note == nil || *entry.Note != "hi" {
		t.Fatalf("entry mismatch: %+v", entry)
	}
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

// TestApplyTenantSave_ResolvesBothMapsInMemory proves a saved entry is
// reflected entirely in memory (no loader/cluster call): the new tenant
// is appended to Tenants and BOTH tenant-owned maps are re-keyed from
// raw suffix → tenant name with a resolved Owner.
func TestApplyTenantSave_ResolvesBothMapsInMemory(t *testing.T) {
	t.Parallel()

	m := makeTestModel()
	m.category = domain.DedicatedAICluster
	m.dataset = &models.Dataset{
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
			"abc": {{Name: "dac1", TenantID: "abc"}}, // unresolved: raw-suffix key, Owner nil
		},
		ImportedModelMap: map[string][]models.ImportedModel{
			"abc": {{BaseModel: models.BaseModel{Name: "im1"}, TenantID: "abc"}},
		},
	}

	m.applyTenantSave(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: strp("acme"), IsInternal: boolp(true),
	})

	// New standalone tenant appended.
	require.Len(t, m.dataset.Tenants, 1)
	require.Equal(t, "acme", m.dataset.Tenants[0].Name)
	require.True(t, m.dataset.Tenants[0].IsInternal)

	// DAC map re-keyed (raw suffix → tenant name) + Owner resolved.
	require.NotContains(t, m.dataset.DedicatedAIClusterMap, "abc")
	dac := m.dataset.DedicatedAIClusterMap["acme"]
	require.Len(t, dac, 1)
	require.NotNil(t, dac[0].Owner)
	require.Equal(t, "acme", dac[0].Owner.Name)

	// ImportedModel map (the sibling category) re-keyed + resolved too.
	imm := m.dataset.ImportedModelMap["acme"]
	require.Len(t, imm, 1)
	require.NotNil(t, imm[0].Owner)
	require.Equal(t, "acme", imm[0].Owner.Name)
}

// TestApplyTenantSave_NilMapsSafe ensures a not-yet-loaded category map
// is skipped without panicking; Tenants still gains the entry.
func TestApplyTenantSave_NilMapsSafe(t *testing.T) {
	t.Parallel()

	m := makeTestModel()
	m.category = domain.DedicatedAICluster
	m.dataset = &models.Dataset{} // both maps nil
	m.applyTenantSave(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: strp("acme"), IsInternal: boolp(false),
	})
	if len(m.dataset.Tenants) != 1 || m.dataset.Tenants[0].Name != "acme" {
		t.Fatalf("Tenants: %+v", m.dataset.Tenants)
	}
}

// TestHandleTenantSavedMsg_AfterFormDismissed proves the success handler
// still applies (toast + in-memory resolution) when the user dismissed
// the form (esc) before the async write landed.
func TestHandleTenantSavedMsg_AfterFormDismissed(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.ListView
	m.editTenant = nil // user already pressed esc
	m.dataset = &models.Dataset{}
	m.category = domain.DedicatedAICluster
	cmd := m.handleTenantSavedMsg(tenantSavedMsg{
		path:  "/tmp/metadata.yaml",
		entry: models.TenantMetadata{ID: "ocid1.tenancy.oc1..abc", Name: strp("acme"), IsInternal: boolp(true)},
	})
	if cmd == nil {
		t.Fatal("expected a toast cmd even when the form was already dismissed")
	}
	if m.viewMode != common.ListView {
		t.Fatalf("viewMode should remain ListView, got %v", m.viewMode)
	}
	if len(m.dataset.Tenants) != 1 {
		t.Fatalf("save should have applied in memory; Tenants=%+v", m.dataset.Tenants)
	}
}
