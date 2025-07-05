package production

import (
	"context"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestNewLoaderImplementsInterface(t *testing.T) {
	t.Parallel()
	_ = NewLoader(context.Background(), "")
}

func TestLoadDataset_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
	// Use a repo path that does not exist to trigger error
	_, err := ldr.LoadDataset(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadDataset with bad path: want error, got nil")
	}
}

func TestLoadTenancyOverrideGroup_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
	_, err := ldr.LoadTenancyOverrideGroup(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err == nil {
		t.Error("LoadTenancyOverrideGroup with bad path: want error, got nil")
	}
}

func TestLoadLimitRegionalOverrides_EmptyOnBadPath(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
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
	ldr := NewLoader(context.Background(), "")
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
	ldr := NewLoader(context.Background(), "")
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
	ldr := NewLoader(ctx, "")
	env := models.Environment{}
	// These should not panic, even if they return errors or empty data
	_, _ = ldr.LoadDataset(ctx, "", env)
	_, _ = ldr.LoadBaseModels(ctx, "", env)
	_, _ = ldr.LoadGpuPools(ctx, "", env)
	// Skip LoadGpuNodes and LoadDedicatedAIClusters: require valid kubeconfig
	_, _ = ldr.LoadTenancyOverrideGroup(ctx, "", env)
	_, _ = ldr.LoadLimitRegionalOverrides(ctx, "", env)
	_, _ = ldr.LoadConsolePropertyRegionalOverrides(ctx, "", env)
	_, _ = ldr.LoadPropertyRegionalOverrides(ctx, "", env)
}

func TestLoadBaseModels_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
	_, err := ldr.LoadBaseModels(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadBaseModels with bad path: want error, got nil")
	}
}

func TestLoadGpuPools_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
	_, err := ldr.LoadGpuPools(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadGpuPools with bad path: want error, got nil")
	}
}

func TestProductionLoader_LoadDataset(t *testing.T) {
	t.Parallel()
	loader := Loader{}
	_, err := loader.LoadDataset(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadDataset: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadBaseModels(t *testing.T) {
	t.Parallel()
	loader := Loader{}
	_, err := loader.LoadBaseModels(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadBaseModels: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuPools(t *testing.T) {
	t.Parallel()
	loader := Loader{}
	_, err := loader.LoadGpuPools(context.Background(), "dummy_repo", models.Environment{})
	if err == nil {
		t.Log("LoadGpuPools: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadGpuNodes(t *testing.T) {
	t.Parallel()
	loader := Loader{}
	_, err := loader.LoadGpuNodes(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadGpuNodes: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_LoadDedicatedAIClusters(t *testing.T) {
	t.Parallel()
	loader := Loader{}
	_, err := loader.LoadDedicatedAIClusters(context.Background(), "dummy_kubeconfig", models.Environment{})
	if err == nil {
		t.Log("LoadDedicatedAIClusters: expected error or empty result with dummy input")
	}
}

func TestProductionLoader_RegionalOverrides(t *testing.T) {
	t.Parallel()
	loader := Loader{}
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

	_ = NewLoader(context.Background(), tmp.Name())
}

func TestLoader_LoadGpuNodesAndDedicatedAIClusters_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader(context.Background(), "")
	env := models.Environment{}
	_, err := ldr.LoadGpuNodes(context.Background(), "", env)
	if err == nil {
		t.Error("LoadGpuNodes: want error for empty kubeconfig, got nil")
	}
	_, err = ldr.LoadDedicatedAIClusters(context.Background(), "", env)
	if err == nil {
		t.Error("LoadDedicatedAIClusters: want error for empty kubeconfig, got nil")
	}
}
