//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stageScaleEnv wires the env triple, kubeconfig, AND repo_path that
// validateScaleConfig requires.
func stageScaleEnv(t *testing.T) {
	t.Helper()
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestScaleGpuPool_DryRun_DoesNotCallOCI(t *testing.T) {
	stageScaleEnv(t)
	called := false
	origInc := increasePoolSizeFn
	defer func() { increasePoolSizeFn = origInc }()
	increasePoolSizeFn = func(context.Context, *models.GpuPool, models.Environment, logging.Logger) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"scale", "gpupool", "pool-a", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would scale gpu_pool/pool-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestScaleGpuPool_HappyPath(t *testing.T) {
	stageScaleEnv(t)
	origResolver := gpuPoolResolverFn
	defer func() { gpuPoolResolverFn = origResolver }()
	gpuPoolResolverFn = func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GpuPool, error) {
		return &models.GpuPool{Name: name, ID: "ocid1.instancepool.fake", Size: 8, ActualSize: 4}, nil
	}

	var gotPool *models.GpuPool
	origInc := increasePoolSizeFn
	defer func() { increasePoolSizeFn = origInc }()
	increasePoolSizeFn = func(_ context.Context, p *models.GpuPool, _ models.Environment, _ logging.Logger) error {
		gotPool = p
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"scale", "gpupool", "pool-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPool == nil || gotPool.Name != "pool-a" || gotPool.ID == "" {
		t.Errorf("unexpected pool handed to IncreasePoolSize: %+v", gotPool)
	}
	if !strings.Contains(out.String(), "scale gpu_pool/pool-a: OK") {
		t.Errorf("expected OK, got: %q", out.String())
	}
}

func TestScaleGpuPool_PoolNotFound(t *testing.T) {
	stageScaleEnv(t)
	origResolver := gpuPoolResolverFn
	defer func() { gpuPoolResolverFn = origResolver }()
	gpuPoolResolverFn = func(context.Context, config.Config, models.Environment, string) (*models.GpuPool, error) {
		return nil, errors.New("gpu pool \"pool-x\" not found in repo")
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"scale", "gpupool", "pool-x", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when pool not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestScaleGpuPool_MissingRepoPath(t *testing.T) {
	// stageMutationEnv only sets env + kubeconfig; no repo_path.
	stageMutationEnv(t)
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"scale", "gpupool", "pool-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when repo_path missing")
	}
	if !strings.Contains(err.Error(), "--repo_path") {
		t.Errorf("error should mention --repo_path: %v", err)
	}
}

func TestScaleGpuPool_InteractiveBail(t *testing.T) {
	stageScaleEnv(t)
	called := false
	origInc := increasePoolSizeFn
	defer func() { increasePoolSizeFn = origInc }()
	increasePoolSizeFn = func(context.Context, *models.GpuPool, models.Environment, logging.Logger) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"scale", "gpupool", "pool-a"})
	cmd.SetIn(strings.NewReader("n\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("must not call OCI after user types n")
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Errorf("expected 'aborted', got: %q", out.String())
	}
}
