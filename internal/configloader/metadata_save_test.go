package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

func TestSaveMetadata_YAMLRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "metadata.yaml") // nested dir must be created
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..abc", Name: strptr("acme"), IsInternal: boolptr(true)},
	}}
	require.NoError(t, SaveMetadata(path, in))
	got, err := LoadMetadata(path)
	require.NoError(t, err)
	require.Len(t, got.Tenants, 1)
	require.Equal(t, "ocid1.tenancy.oc1..abc", got.Tenants[0].ID)
	require.NotNil(t, got.Tenants[0].Name)
	require.Equal(t, "acme", *got.Tenants[0].Name)
}

func TestSaveMetadata_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.json")
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..xyz", Name: strptr("beta"), IsInternal: boolptr(false)},
	}}
	require.NoError(t, SaveMetadata(path, in))
	got, err := LoadMetadata(path)
	require.NoError(t, err)
	require.Len(t, got.Tenants, 1)
	require.NotNil(t, got.Tenants[0].Name)
	require.Equal(t, "beta", *got.Tenants[0].Name)
}

// Finding #8: SaveMetadata must be atomic — if the new content cannot be
// written, the existing file is left intact rather than truncated/corrupted.
func TestSaveMetadata_AtomicPreservesOnWriteFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("read-only directory permissions are not enforced for root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	orig := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "id-orig"}}}
	require.NoError(t, SaveMetadata(path, orig))

	// Make the directory unwritable: a new (temp) file cannot be created, so an
	// atomic save must fail without disturbing the already-written file.
	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // G302: read-only dir is the point of the test; dirs need the exec bit
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restoring dir perms so TempDir cleanup can remove it

	updated := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "id-updated"}}}
	require.Error(t, SaveMetadata(path, updated))

	// Original content must survive untouched.
	got, err := LoadMetadata(path)
	require.NoError(t, err)
	require.Len(t, got.Tenants, 1)
	require.Equal(t, "id-orig", got.Tenants[0].ID)
}

func TestSaveMetadata_UnsupportedExt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.txt")
	require.Error(t, SaveMetadata(path, &models.Metadata{}))
}

func TestUpsertTenant_AppendThenReplace(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{}
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A"), IsInternal: boolptr(true)}))
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-b", Name: strptr("B"), IsInternal: boolptr(false)}))
	require.Len(t, m.Tenants, 2)
	// Replace id-a in place.
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), IsInternal: boolptr(false)}))
	require.Len(t, m.Tenants, 2, "replace should not append")
	require.Equal(t, "id-a", m.Tenants[0].ID)
	require.NotNil(t, m.Tenants[0].Name)
	require.Equal(t, "A2", *m.Tenants[0].Name)
}

// Names map one to one to tenancies: an upsert giving a second tenancy an
// existing name is refused and leaves m unchanged; keeping one's own name
// is fine.
func TestUpsertTenant_RejectsTakenName(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "id-a", Name: strptr("Acme")}}}
	err := UpsertTenant(m, models.TenantMetadata{ID: "id-b", Name: strptr("Acme")})
	require.ErrorContains(t, err, `"Acme" is already used by id-a`)
	require.Len(t, m.Tenants, 1)
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("Acme"), Note: strptr("n")}))
}

// Names are stored trimmed and compared trimmed: "Acme " doesn't get
// past the check as a second Acme, and a blank name is refused.
func TestUpsertTenant_TrimsName(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "ocid1.tenancy.oc1..a", Name: strptr("Acme")}}}
	err := UpsertTenant(m, models.TenantMetadata{ID: "ocid1.tenancy.oc1..b", Name: strptr("Acme ")})
	require.ErrorContains(t, err, `"Acme" is already used by ocid1.tenancy.oc1..a`)

	require.ErrorContains(t, UpsertTenant(m, models.TenantMetadata{ID: "ocid1.tenancy.oc1..b", Name: strptr("   ")}), "blank")
	require.Len(t, m.Tenants, 1)

	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "ocid1.tenancy.oc1..b", Name: strptr("  Beta\t")}))
	assert.Equal(t, "Beta", *m.Tenants[1].Name)
}

// A name another realm's tenancy has is not taken.
func TestUpsertTenant_NamesPerRealm(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "ocid1.tenancy.oc1..a", Name: strptr("Acme")}}}
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "ocid1.tenancy.oc4..b", Name: strptr("Acme")}))
	require.Len(t, m.Tenants, 2)
}

// A metadata file giving two tenancies one name, or listing a tenancy
// twice, fails to load.
func TestLoadMetadata_RejectsDuplicates(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"name": "tenants:\n- {id: id-a, name: Acme}\n- {id: id-b, name: Acme}\n",
		"id":   "tenants:\n- {id: id-a, name: A}\n- {id: id-a, name: B}\n",
	} {
		path := filepath.Join(t.TempDir(), "metadata.yaml")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		_, err := LoadMetadata(path)
		require.ErrorContainsf(t, err, "invalid tenant metadata", "duplicate %s", name)
	}
}

// Status is machine-owned: an edit that doesn't carry one (the TUI form,
// MCP upsert_tenant_metadata) keeps it, while the sync's own upsert,
// which does carry one, replaces it.
func TestUpsertTenant_KeepsStatus(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "id-a", Name: strptr("A"), Status: "ACTIVE"}}}
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2")}))
	require.Equal(t, "ACTIVE", m.Tenants[0].Status)
	require.Equal(t, "A2", *m.Tenants[0].Name)
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), Status: "DELETING"}))
	require.Equal(t, "DELETING", m.Tenants[0].Status)
}

// An edit without an internal flag (`set tenant` without --internal) keeps
// the recorded one, e.g. the flag `tenant sync` set; one with it replaces it.
func TestUpsertTenant_KeepsInternalFlag(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{Tenants: []models.TenantMetadata{{ID: "id-a", Name: strptr("A"), IsInternal: boolptr(false)}}}
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2")}))
	require.Equal(t, boolptr(false), m.Tenants[0].IsInternal)
	require.NoError(t, UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), IsInternal: boolptr(true)}))
	require.Equal(t, boolptr(true), m.Tenants[0].IsInternal)
}
