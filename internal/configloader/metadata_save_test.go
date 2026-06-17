package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

func TestSaveMetadata_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "metadata.yaml") // nested dir must be created
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..abc", Name: strptr("acme"), IsInternal: boolptr(true)},
	}}
	if err := SaveMetadata(path, in); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}
	got, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 || got.Tenants[0].ID != "ocid1.tenancy.oc1..abc" ||
		got.Tenants[0].Name == nil || *got.Tenants[0].Name != "acme" {
		t.Fatalf("round-trip mismatch: %+v", got.Tenants)
	}
}

func TestSaveMetadata_JSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..xyz", Name: strptr("beta"), IsInternal: boolptr(false)},
	}}
	if err := SaveMetadata(path, in); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}
	got, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 || got.Tenants[0].Name == nil || *got.Tenants[0].Name != "beta" {
		t.Fatalf("round-trip mismatch: %+v", got.Tenants)
	}
}

func TestSaveMetadata_UnsupportedExt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.txt")
	if err := SaveMetadata(path, &models.Metadata{}); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestUpsertTenant_AppendThenReplace(t *testing.T) {
	m := &models.Metadata{}
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A"), IsInternal: boolptr(true)})
	UpsertTenant(m, models.TenantMetadata{ID: "id-b", Name: strptr("B"), IsInternal: boolptr(false)})
	if len(m.Tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(m.Tenants))
	}
	// Replace id-a in place.
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), IsInternal: boolptr(false)})
	if len(m.Tenants) != 2 {
		t.Fatalf("replace should not append: got %d", len(m.Tenants))
	}
	if m.Tenants[0].ID != "id-a" || m.Tenants[0].Name == nil || *m.Tenants[0].Name != "A2" {
		t.Fatalf("expected id-a replaced in place: %+v", m.Tenants[0])
	}
	_ = os.Stdout // keep os import if trimmed by tooling
}
