package production

import (
	"context"
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
