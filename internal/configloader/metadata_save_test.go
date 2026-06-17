package configloader

import (
	"path/filepath"
	"testing"

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

func TestSaveMetadata_UnsupportedExt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.txt")
	require.Error(t, SaveMetadata(path, &models.Metadata{}))
}

func TestUpsertTenant_AppendThenReplace(t *testing.T) {
	t.Parallel()
	m := &models.Metadata{}
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A"), IsInternal: boolptr(true)})
	UpsertTenant(m, models.TenantMetadata{ID: "id-b", Name: strptr("B"), IsInternal: boolptr(false)})
	require.Len(t, m.Tenants, 2)
	// Replace id-a in place.
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), IsInternal: boolptr(false)})
	require.Len(t, m.Tenants, 2, "replace should not append")
	require.Equal(t, "id-a", m.Tenants[0].ID)
	require.NotNil(t, m.Tenants[0].Name)
	require.Equal(t, "A2", *m.Tenants[0].Name)
}
