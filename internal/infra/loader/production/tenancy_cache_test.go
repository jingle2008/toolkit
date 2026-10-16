package production

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

// writeLimitOverride writes one limit tenancy override for tenantID under
// the tenant directory dir, creating the other two override kinds' realm
// directories so the group loads.
func writeLimitOverride(t *testing.T, repo, dir, tenantID string) {
	t.Helper()
	limits := filepath.Join(repo, "shared_modules", "limits")
	for _, kind := range []string{"console_properties", "properties"} {
		require.NoError(t, os.MkdirAll(filepath.Join(limits, kind+"_tenancy_overrides", "regional_values", "oc1"), 0o750))
	}
	tenantDir := filepath.Join(limits, "limits_tenancy_overrides", "regional_values", "oc1", dir)
	require.NoError(t, os.MkdirAll(tenantDir, 0o750))
	data, err := json.Marshal(models.LimitTenancyOverride{TenantID: tenantID})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tenantDir, "limits.json"), data, 0o600))
}

func tenantNames(g models.TenancyOverrideGroup) []string {
	names := make([]string, len(g.Tenants))
	for i, tn := range g.Tenants {
		names[i] = tn.Name
	}
	return names
}

// The group is reused while nothing changes, and reloaded once an
// override file or the tenant metadata does.
func TestLoadTenancyOverrideGroup_Cache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := t.TempDir()
	writeLimitOverride(t, repo, "t1", "ocid1.tenancy.oc1..a")
	ldr := New(ctx, filepath.Join(t.TempDir(), "metadata.yaml"))
	l, ok := ldr.(*Client)
	require.True(t, ok)
	env := models.Environment{Realm: "oc1"}

	first, err := l.LoadTenancyOverrideGroup(ctx, repo, env)
	require.NoError(t, err)
	assert.Equal(t, []string{"ocid1.tenancy.oc1..a"}, tenantNames(first), "no metadata name: named by OCID")
	again, err := l.LoadTenancyOverrideGroup(ctx, repo, env)
	require.NoError(t, err)
	assert.Same(t, &first.Tenants[0], &again.Tenants[0], "unchanged: the cached group is reused")

	writeLimitOverride(t, repo, "t2", "ocid1.tenancy.oc1..b")
	grown, err := l.LoadTenancyOverrideGroup(ctx, repo, env)
	require.NoError(t, err)
	assert.Equal(t, []string{"ocid1.tenancy.oc1..a", "ocid1.tenancy.oc1..b"}, tenantNames(grown), "a new override file reloads")

	name := "Acme"
	require.NoError(t, l.UpsertTenantMetadata(models.TenantMetadata{ID: "ocid1.tenancy.oc1..a", Name: &name}))
	renamed, err := l.LoadTenancyOverrideGroup(ctx, repo, env)
	require.NoError(t, err)
	assert.Equal(t, []string{"Acme", "ocid1.tenancy.oc1..b"}, tenantNames(renamed), "a metadata write reloads")
}

// A Client built without New has no cache and still loads.
func TestLoadTenancyOverrideGroup_ZeroClient(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeLimitOverride(t, repo, "t1", "ocid1.tenancy.oc1..a")
	g, err := Client{metadata: &models.Metadata{}}.LoadTenancyOverrideGroup(context.Background(), repo, models.Environment{Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"ocid1.tenancy.oc1..a"}, tenantNames(g))
}

// Metadata is the only source of tenant names, so a metadata file that
// fails validation blocks loading tenants instead of silently naming
// every tenant by its OCID.
func TestLoadTenancyOverrideGroup_InvalidMetadataBlocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := t.TempDir()
	writeLimitOverride(t, repo, "t1", "ocid1.tenancy.oc1..a")
	meta := filepath.Join(t.TempDir(), "metadata.yaml")
	require.NoError(t, os.WriteFile(meta, []byte("tenants:\n- {id: ocid1.tenancy.oc1..a, name: Acme}\n- {id: ocid1.tenancy.oc1..b, name: Acme}\n"), 0o600))
	ldr := New(ctx, meta)

	_, err := ldr.LoadTenancyOverrideGroup(ctx, repo, models.Environment{Realm: "oc1"})
	require.ErrorContains(t, err, `tenant name "Acme" is used by both`)
	_, err = ldr.LoadDataset(ctx, repo, models.Environment{Realm: "oc1"})
	require.ErrorContains(t, err, "invalid tenant metadata")
}
