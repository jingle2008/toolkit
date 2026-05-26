//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
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
	called := false
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, models.Environment, logging.Logger) error {
		called = true
		return nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out, "DRY-RUN: would scale gpu_pool/pool-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestScaleGPUPool_HappyPath(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&gpuPoolResolverFn, func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: name, ID: "ocid1.instancepool.fake", Size: 8, ActualSize: 4}, nil
	})()

	var gotPool *models.GPUPool
	defer swap(&increasePoolSizeFn, func(_ context.Context, p *models.GPUPool, _ models.Environment, _ logging.Logger) error {
		gotPool = p
		return nil
	})()

	out, err := runRootCmd(t, []string{"scale", "gpupool", "pool-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPool == nil || gotPool.Name != "pool-a" || gotPool.ID == "" {
		t.Errorf("unexpected pool handed to IncreasePoolSize: %+v", gotPool)
	}
	if !strings.Contains(out, "scale gpu_pool/pool-a: OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestScaleGPUPool_PoolNotFound(t *testing.T) {
	stageScaleEnv(t)
	defer swap(&gpuPoolResolverFn, func(context.Context, config.Config, models.Environment, string) (*models.GPUPool, error) {
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
	called := false
	defer swap(&increasePoolSizeFn, func(context.Context, *models.GPUPool, models.Environment, logging.Logger) error {
		called = true
		return nil
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
