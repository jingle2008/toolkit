package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
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
	f.name.SetValue("   ")
	if _, ok := f.toEntry(); ok {
		t.Fatal("a name of spaces must be rejected")
	}
	f.name.SetValue(" acme ")
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

// TestApplyTenantSave_ResolvesNonTenantKeyedMaps covers the maps grouped by
// something other than the tenant: GPUWorkload (node) and InferenceService
// (namespace) keep their keys and gain the resolved Owner.
func TestApplyTenantSave_ResolvesNonTenantKeyedMaps(t *testing.T) {
	t.Parallel()

	m := makeTestModel()
	m.category = domain.GPUWorkload
	m.dataset = &models.Dataset{
		GPUWorkloadMap: map[string][]models.GPUWorkload{
			"node-a": {{Name: "pod1", Node: "node-a", TenantID: "abc"}},
		},
		InferenceServiceMap: map[string][]models.InferenceService{
			"amaaaaaadac1": {{Name: "e1", Namespace: "amaaaaaadac1", TenantID: "abc"}},
		},
	}

	m.applyTenantSave(models.TenantMetadata{ID: "ocid1.tenancy.oc1..abc", Name: strp("acme")})

	gw := m.dataset.GPUWorkloadMap["node-a"]
	require.Len(t, gw, 1)
	require.NotNil(t, gw[0].Owner)
	require.Equal(t, "acme", gw[0].TenantName())

	is := m.dataset.InferenceServiceMap["amaaaaaadac1"]
	require.Len(t, is, 1)
	require.NotNil(t, is[0].Owner)
	require.Equal(t, "acme", is[0].TenantName())
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
		realm: m.environment.Realm,
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

// A save that lands after a switch to another realm is not applied to that
// realm's tenants: tenants are per realm, and the saved one belongs to the
// realm it was saved in (it loads from the file when that realm is shown).
func TestHandleTenantSavedMsg_OtherRealmNotApplied(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.environment.Realm = "oc2"
	m.dataset = &models.Dataset{}
	cmd := m.handleTenantSavedMsg(tenantSavedMsg{
		path:  "/tmp/metadata.yaml",
		entry: models.TenantMetadata{ID: "ocid1.tenancy.oc1..abc", Name: strp("acme"), IsInternal: boolp(true)},
		realm: "oc1",
	})
	if cmd == nil {
		t.Fatal("expected the saved toast")
	}
	if len(m.dataset.Tenants) != 0 {
		t.Fatalf("an oc1 tenant must not join oc2's tenants: %+v", m.dataset.Tenants)
	}
}

func TestIsToggleKey(t *testing.T) {
	t.Parallel()
	for _, k := range []string{" ", "space", "left", "right"} {
		assert.True(t, isToggleKey(k), k)
	}
	for _, k := range []string{"tab", "x", "enter"} {
		assert.False(t, isToggleKey(k), k)
	}
}

// Every category with a Tenant column binds E (edit tenant).
func TestEditTenant_BoundWhereverTenantColumn(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories() {
		hasTenant := false
		for _, h := range headersFor(cat) {
			hasTenant = hasTenant || h.text == common.TenantCol
		}
		bound := false
		for _, b := range keys.ResolveKeys(cat, common.ListView).Context {
			bound = bound || b.Help() == keys.EditTenant.Help()
		}
		assert.Equalf(t, hasTenant, bound, "%s: Tenant column %v, E bound %v", cat, hasTenant, bound)
	}
}

// The newly covered rows qualify while their tenant has no metadata name:
// tenant-owned rows whose tenancy resolves to no tenant, and tenancy
// overrides whose tenant is still named by its OCID.
func TestTenantEditTarget_EveryTenantColumn(t *testing.T) {
	t.Parallel()
	const ocid = "ocid1.tenancy.oc1..xyz"
	acme := &models.Tenant{Name: "acme", ID: ocid}
	for name, tc := range map[string]struct {
		item any
		ok   bool
	}{
		"workload unresolved":       {&models.GPUWorkload{TenantID: "xyz"}, true},
		"workload resolved":         {&models.GPUWorkload{TenantID: "xyz", Owner: acme}, false},
		"endpoint unresolved":       {&models.InferenceService{TenantID: "xyz"}, true},
		"endpoint no tenancy":       {&models.InferenceService{}, false},
		"limit override unnamed":    {&models.LimitTenancyOverride{TenantID: ocid, TenantName: ocid}, true},
		"limit override named":      {&models.LimitTenancyOverride{TenantID: ocid, TenantName: "acme"}, false},
		"console override unnamed":  {&models.ConsolePropertyTenancyOverride{TenantID: ocid, TenantName: ocid}, true},
		"property override unnamed": {&models.PropertyTenancyOverride{TenantID: ocid, TenantName: ocid}, true},
		"property override no id":   {&models.PropertyTenancyOverride{}, false},
	} {
		tgt, ok := tenantEditTarget(tc.item, "oc1")
		assert.Equal(t, tc.ok, ok, name)
		if tc.ok {
			assert.Equal(t, ocid, tgt.ocid, name)
		}
	}
}

// Naming a tenant an override's OCID made renames it: its overrides move
// from under the OCID to the name, and carry it.
func TestApplyTenantSave_RenamesOverrideTenant(t *testing.T) {
	t.Parallel()
	const ocid = "ocid1.tenancy.oc1..xyz"
	m := newTestModel(t)
	m.dataset = &models.Dataset{
		Tenants: []models.Tenant{{ID: ocid, Name: ocid, IsInternal: true}},
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
			ocid: {{TenantID: ocid, TenantName: ocid, LimitRegionalOverride: models.LimitRegionalOverride{Name: "l1"}}},
		},
		PropertyTenancyOverrideMap: map[string][]models.PropertyTenancyOverride{
			ocid: {{TenantID: ocid, TenantName: ocid}},
		},
	}
	name := "Acme"
	m.applyTenantSave(models.TenantMetadata{ID: ocid, Name: &name})

	require.Len(t, m.dataset.Tenants, 1)
	assert.Equal(t, "Acme", m.dataset.Tenants[0].Name)
	assert.NotContains(t, m.dataset.LimitTenancyOverrideMap, ocid)
	require.Len(t, m.dataset.LimitTenancyOverrideMap["Acme"], 1)
	assert.Equal(t, "Acme", m.dataset.LimitTenancyOverrideMap["Acme"][0].TenantName)
	assert.Equal(t, "Acme", m.dataset.PropertyTenancyOverrideMap["Acme"][0].TenantName)
	assert.Nil(t, m.dataset.ConsolePropertyTenancyOverrideMap, "an unloaded map stays unloaded")
}
