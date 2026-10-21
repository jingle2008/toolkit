//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stageScaleEnv wires the env triple, kubeconfig, AND repo_path that
// validateScaleConfig requires. stageMutationEnv already calls
// viper.Reset and registers the cleanup — adding TOOLKIT_REPO_PATH
// via t.Setenv is enough because viper picks it up via AutomaticEnv
// at Unmarshal time.
func stageScaleEnv(t *testing.T) {
	t.Helper()
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
}

func TestScaleGPUPool_DryRun_DoesNotCallOCI(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 8, ActualSize: 4}, nil
	})()
	called := false
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, int, models.Environment, logging.Logger) (oci.PoolScale, error) {
		called = true
		return oci.PoolScale{}, nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	// The pool is looked up, as MCP's dry_run does, and the line says
	// what was found.
	if !strings.Contains(out, "DRY-RUN: would scale gpu_pool/pool-a (actual_size=4, add=4, pool_id=ocid1.instancepool.fake, size=8, target_size=8)") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestScaleGPUPool_HappyPath(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 8, ActualSize: 4}, nil
	})()

	var (
		gotPool *models.GPUPool
		gotAdd  int
	)
	defer swap(&increasePoolSizeFn, func(_ context.Context, p *models.GPUPool, add int, _ models.Environment, _ logging.Logger) (oci.PoolScale, error) {
		gotPool, gotAdd = p, add
		return oci.PoolScale{Shown: 4, Before: 4, After: 4 + add}, nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPool == nil || gotPool.Name != "pool-a" || gotPool.ID == "" {
		t.Errorf("unexpected pool handed to IncreasePoolSize: %+v", gotPool)
	}
	if gotAdd != 4 {
		t.Errorf("without --add a short pool is filled: add=%d, want 4", gotAdd)
	}
	if !strings.Contains(out, "scale gpu_pool/pool-a: OK (4 → 8 instances)") {
		t.Errorf("expected OK with sizes, got: %q", out)
	}
}

// --add goes past the Terraform size, and the output warns that
// terraform apply undoes it.
func TestScaleGPUPool_AddPastTerraformSize(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 4, ActualSize: 4}, nil
	})()
	gotAdd := 0
	defer swap(&increasePoolSizeFn, func(_ context.Context, _ *models.GPUPool, add int, _ models.Environment, _ logging.Logger) (oci.PoolScale, error) {
		gotAdd = add
		return oci.PoolScale{Shown: 4, Before: 4, After: 4 + add}, nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "--add", "2", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotAdd != 2 {
		t.Errorf("add=%d, want 2", gotAdd)
	}
	for _, want := range []string{
		"Grows the instance pool from 4 to 6 instances.",
		"Terraform declares 4; the next terraform apply shrinks it back",
		"scale gpu_pool/pool-a: OK (4 → 6 instances)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

// Without --add, a pool at its Terraform size is left alone and says
// how to go past it: a script running this after terraform apply keeps
// its meaning.
func TestScaleGPUPool_AtSizeWithoutAddSkips(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 4, ActualSize: 4}, nil
	})()
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, int, models.Environment, logging.Logger) (oci.PoolScale, error) {
		t.Fatal("must not call OCI")
		return oci.PoolScale{}, nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "already at its Terraform size (4); nothing to scale. Pass --add N") {
		t.Errorf("expected skip note, got: %q", out)
	}
}

func TestScaleGPUPool_Refusals(t *testing.T) {
	stageScaleEnv(t)
	pool := models.GPUPool{Name: "pool-a", Size: 4, Status: models.GPUPoolStatusNonexistent}
	defer swap(&resolveGPUPoolFn, func(context.Context, config.Config, models.Environment, string) (*models.GPUPool, error) {
		p := pool
		return &p, nil
	})()
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, int, models.Environment, logging.Logger) (oci.PoolScale, error) {
		t.Fatal("must not call OCI")
		return oci.PoolScale{}, nil
	})()

	_, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "-y"}, "")
	if !errors.Is(err, oci.ErrNoInstancePool) {
		t.Errorf("a pool OCI lacks: got %v", err)
	}
	pool.ID, pool.Status, pool.ActualSize = "ocid1.instancepool.fake", "RUNNING", 4
	_, err = runRootCmd(t, []string{"scale", "gpupool", "pool-a", "--add", "0", "-y"}, "")
	if err == nil || !strings.Contains(err.Error(), "at least 1") {
		t.Errorf("--add 0 must be refused: got %v", err)
	}
}

func TestScaleGPUPool_PoolNotFound(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(context.Context, config.Config, models.Environment, string) (*models.GPUPool, error) {
		return nil, errors.New("gpu pool \"pool-x\" not found in repo")
	})()

	_, err := runRootCmd(t, []string{"scale", "gpupool", "pool-x", "-y"}, "")
	if err == nil {
		t.Fatal("expected error when pool not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestScaleGPUPool_MissingRepoPath(t *testing.T) {
	// stageMutationEnv only sets env + kubeconfig; no repo_path.
	stageMutationEnv(t)
	_, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "-y"}, "")
	if err == nil {
		t.Fatal("expected error when repo_path missing")
	}
	if !strings.Contains(err.Error(), "--repo-path") {
		t.Errorf("error should mention --repo-path: %v", err)
	}
}

func TestScaleGPUPool_InteractiveBail(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&resolveGPUPoolFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 8, ActualSize: 4}, nil
	})()
	called := false
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, int, models.Environment, logging.Logger) (oci.PoolScale, error) {
		called = true
		return oci.PoolScale{}, nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a"}, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("must not call OCI after user types n")
	}
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted', got: %q", out)
	}
}
