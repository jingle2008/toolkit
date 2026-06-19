package production

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }

func TestNewLoaderImplementsInterface(t *testing.T) {
	t.Parallel()
	_ = New(context.Background(), "")
}

func TestLoadDataset_Error(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	// Use a repo path that does not exist to trigger error
	_, err := ldr.LoadDataset(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadDataset with bad path: want error, got nil")
	}
}

func TestLoadTenancyOverrideGroup_Error(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	_, err := ldr.LoadTenancyOverrideGroup(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err == nil {
		t.Error("LoadTenancyOverrideGroup with bad path: want error, got nil")
	}
}

func TestLoadLimitRegionalOverrides_EmptyOnBadPath(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	got, err := ldr.LoadLimitRegionalOverrides(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}

func TestLoadConsolePropertyRegionalOverrides_EmptyOnBadPath(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	got, err := ldr.LoadConsolePropertyRegionalOverrides(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}

func TestLoadPropertyRegionalOverrides_EmptyOnBadPath(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	got, err := ldr.LoadPropertyRegionalOverrides(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}

func TestLoader_AllMethods_NoPanicOnEmptyInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ldr := New(ctx, "")
	env := models.Environment{}
	// These should not panic, even if they return errors or empty data
	_, _ = ldr.LoadDataset(ctx, "", env)
	_, _ = ldr.LoadBaseModels(ctx, "", env)
	_, _ = ldr.LoadGPUPools(ctx, "", env)
	// Skip LoadGPUNodesByPool and LoadDedicatedAIClusters: require valid kubeconfig
	_, _ = ldr.LoadTenancyOverrideGroup(ctx, "", env)
	_, _ = ldr.LoadLimitRegionalOverrides(ctx, "", env)
	_, _ = ldr.LoadConsolePropertyRegionalOverrides(ctx, "", env)
	_, _ = ldr.LoadPropertyRegionalOverrides(ctx, "", env)
}

func TestLoadBaseModels_Error(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	_, err := ldr.LoadBaseModels(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadBaseModels with bad path: want error, got nil")
	}
}

func TestLoadGPUPools_Error(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	_, err := ldr.LoadGPUPools(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadGPUPools with bad path: want error, got nil")
	}
}

func TestProductionLoader_LoadDataset(t *testing.T) {
	t.Parallel()
	loader := Client{}
	_, err := loader.LoadDataset(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadDataset: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadBaseModels(t *testing.T) {
	t.Parallel()
	loader := Client{}
	_, err := loader.LoadBaseModels(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadBaseModels: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGPUPools(t *testing.T) {
	t.Parallel()
	loader := Client{}
	_, err := loader.LoadGPUPools(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadGPUPools: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGPUNodes(t *testing.T) {
	t.Parallel()
	loader := Client{}
	_, err := loader.LoadGPUNodesByPool(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadGPUNodesByPool: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadDedicatedAIClusters(t *testing.T) {
	t.Parallel()
	loader := Client{}
	_, err := loader.LoadDedicatedAIClusters(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadDedicatedAIClusters: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_RegionalOverrides(t *testing.T) {
	t.Parallel()
	loader := Client{}
	ctx := context.Background()
	repo := "dummy_repo"
	env := models.Environment{}

	_, err := loader.LoadTenancyOverrideGroup(ctx, repo, env)
	if err == nil {
		t.Log("LoadTenancyOverrideGroup: expected error or empty result with dummy input")
	}

	_, err = loader.LoadLimitRegionalOverrides(ctx, repo, env)
	if err == nil {
		t.Log("LoadLimitRegionalOverrides: expected error or empty result with dummy input")
	}

	_, err = loader.LoadConsolePropertyRegionalOverrides(ctx, repo, env)
	if err == nil {
		t.Log("LoadConsolePropertyRegionalOverrides: expected error or empty result with dummy input")
	}

	_, err = loader.LoadPropertyRegionalOverrides(ctx, repo, env)
	if err == nil {
		t.Log("LoadPropertyRegionalOverrides: expected error or empty result with dummy input")
	}
}

func TestNewLoader_LoadsMetadataFile(t *testing.T) {
	t.Parallel()
	tmp, err := os.CreateTemp(t.TempDir(), "meta-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString("{}")
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	_ = tmp.Close()

	_ = New(context.Background(), tmp.Name())
}

func TestLoader_LoadGPUNodesAndDedicatedAIClusters_Error(t *testing.T) {
	t.Parallel()
	ldr := New(context.Background(), "")
	env := models.Environment{}
	_, err := ldr.LoadGPUNodesByPool(context.Background(), "", env)
	if err == nil {
		t.Error("LoadGPUNodesByPool: want error for empty kubeconfig, got nil")
	}
	_, err = ldr.LoadDedicatedAIClusters(context.Background(), "", env)
	if err == nil {
		t.Error("LoadDedicatedAIClusters: want error for empty kubeconfig, got nil")
	}
}

func TestNew_ImplementsTenantMetadataWriter(t *testing.T) {
	t.Parallel()
	ld := New(context.Background(), "")
	if _, ok := ld.(loader.TenantMetadataWriter); !ok {
		t.Fatal("production.New(...) must satisfy loader.TenantMetadataWriter")
	}
}

func TestUpsertTenantMetadata_WritesAndReplaces(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	w, ok := New(context.Background(), path).(loader.TenantMetadataWriter)
	if !ok {
		t.Fatal("production.New(...) must satisfy loader.TenantMetadataWriter")
	}

	if err := w.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: sp("acme"), IsInternal: bp(true),
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := w.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: sp("acme-renamed"), IsInternal: bp(false),
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := configloader.LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 {
		t.Fatalf("want 1 tenant (replace, not append), got %d", len(got.Tenants))
	}
	if got.Tenants[0].Name == nil || *got.Tenants[0].Name != "acme-renamed" {
		t.Fatalf("want replaced name, got %+v", got.Tenants[0])
	}
	if got.Tenants[0].IsInternal == nil || *got.Tenants[0].IsInternal {
		t.Fatalf("want IsInternal replaced to false, got %+v", got.Tenants[0])
	}
}

func TestUpsertTenantMetadata_PreservesExistingEntries(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	// Seed an on-disk file with two tenants.
	seed := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..aaa", Name: sp("one"), IsInternal: bp(true)},
		{ID: "ocid1.tenancy.oc1..bbb", Name: sp("two"), IsInternal: bp(false)},
	}}
	require.NoError(t, configloader.SaveMetadata(path, seed))

	ld, ok := New(context.Background(), path).(loader.TenantMetadataWriter)
	require.True(t, ok)
	require.NoError(t, ld.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..ccc", Name: sp("three"), IsInternal: bp(true),
	}))

	got, err := configloader.LoadMetadata(path)
	require.NoError(t, err)
	require.Len(t, got.Tenants, 3) // existing two preserved + new one
}

func TestUpsertTenantMetadata_RefusesToOverwriteUnparseableFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	// Write content that is NOT valid for the Metadata schema / YAML.
	const corrupt = "this: : not: valid: yaml\n\t- broken"
	require.NoError(t, os.WriteFile(path, []byte(corrupt), 0o600))

	// Precondition: the corpus must actually fail to parse, otherwise
	// metadataLoadErr would never be set and this test would pass for
	// the wrong reason.
	_, lerr := configloader.LoadMetadata(path)
	require.Error(t, lerr)

	ld, ok := New(context.Background(), path).(loader.TenantMetadataWriter)
	require.True(t, ok)
	err := ld.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..ccc", Name: sp("three"), IsInternal: bp(true),
	})
	require.Error(t, err) // must refuse rather than clobber

	// The corrupt file must be left untouched (not overwritten).
	raw, readErr := os.ReadFile(path) // #nosec G304 -- path is a test-controlled t.TempDir() file
	require.NoError(t, readErr)
	require.Equal(t, corrupt, string(raw))
}

func TestUpsertTenantMetadata_NoPathErrors(t *testing.T) {
	t.Parallel()
	w, ok := New(context.Background(), "").(loader.TenantMetadataWriter)
	if !ok {
		t.Fatal("production.New(...) must satisfy loader.TenantMetadataWriter")
	}
	if err := w.UpsertTenantMetadata(models.TenantMetadata{ID: "x", Name: sp("y"), IsInternal: bp(true)}); err == nil {
		t.Fatal("expected error when no metadata file is configured")
	}
}
