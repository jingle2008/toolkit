package tui

import (
	"testing"

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
