package production

import (
	"context"
	"testing"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestNewLoaderImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ loader.Loader = NewLoader()
}

func TestLoadDataset_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader()
	// Use a repo path that does not exist to trigger error
	_, err := ldr.LoadDataset(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadDataset with bad path: want error, got nil")
	}
}

func TestLoadTenancyOverrideGroup_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader()
	_, err := ldr.LoadTenancyOverrideGroup(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err == nil {
		t.Error("LoadTenancyOverrideGroup with bad path: want error, got nil")
	}
}

func TestLoadLimitRegionalOverrides_EmptyOnBadPath(t *testing.T) {
	t.Parallel()
	ldr := NewLoader()
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
	ldr := NewLoader()
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
	ldr := NewLoader()
	got, err := ldr.LoadPropertyRegionalOverrides(context.Background(), "/nonexistent/path", models.Environment{Realm: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty result, got %d", len(got))
	}
}

// Optionally, add more tests for LoadGpuPools, LoadGpuNodes, etc., using mocks or testutil if available.

func TestLoadBaseModels_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader()
	_, err := ldr.LoadBaseModels(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadBaseModels with bad path: want error, got nil")
	}
}

func TestLoadGpuPools_Error(t *testing.T) {
	t.Parallel()
	ldr := NewLoader()
	_, err := ldr.LoadGpuPools(context.Background(), "/nonexistent/path", models.Environment{})
	if err == nil {
		t.Error("LoadGpuPools with bad path: want error, got nil")
	}
}
