//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestRestartNodeCmd_DryRun_DoesNotCallOCI(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	called := false
	d.SoftReset = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		called = true
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "node", "node-a", "--ocid", "ocid1.instance.fake", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out, "DRY-RUN: would restart node/ocid1.instance.fake (gpu_pool=pool-a, instance_id=ocid1.instance.fake)") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestRestartNodeCmd_OcidNeedsKubeconfig(t *testing.T) {
	d := &ops.Deps{}
	// Stage env WITHOUT a kubeconfig — the --ocid path needs it for the
	// env's GPU compartment, which the instance must be in.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TOOLKIT_KUBECONFIG", "/nonexistent/kubeconfig")
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
	t.Setenv("TOOLKIT_ENV_TYPE", "dev")
	t.Setenv("TOOLKIT_ENV_REGION", "us-ashburn-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc1")
	viper.Reset()
	t.Cleanup(viper.Reset)

	d.ResolveGPUNode = func(context.Context, models.Environment, string, string) (*models.GPUNode, error) {
		t.Error("must not resolve without a kubeconfig")
		return nil, errors.New("unreachable")
	}

	_, err := runRootCmdWith(t, d, []string{"restart", "node", "node-a", "--ocid", "ocid1.instance.fake", "-y"}, "")
	if err == nil || !strings.Contains(err.Error(), "kubeconfig") {
		t.Fatalf("expected unreadable kubeconfig, got: %v", err)
	}
}

func TestRestartNodeCmd_OcidNeedsRepo(t *testing.T) {
	stageMutationEnv(t) // no repo path
	_, err := runRootCmd(t, []string{"restart", "node", "node-a", "--ocid", "ocid1.instance.fake", "-y"})
	if err == nil || !strings.Contains(err.Error(), "--repo-path") {
		t.Fatalf("expected missing --repo-path, got: %v", err)
	}
}

// The prompt names the OCID the restart acts on, not the free-form label.
func TestRestartNodeCmd_OcidPromptNamesOCID(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	called := false
	d.SoftReset = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		called = true
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "node", "node-a", "--ocid", "ocid1.instance.fake"}, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "Confirm restart node/ocid1.instance.fake? [y/N]: ") {
		t.Errorf("prompt must name the OCID, got: %q", out)
	}
	if called {
		t.Fatal("declined prompt must not call OCI")
	}
}

// An OCID the lookup can't place in one of the env's GPU pools is
// refused before the prompt, and nothing is called.
func TestRestartNodeCmd_OcidNotInGPUPool(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	d.ResolveGPUNode = func(context.Context, models.Environment, string, string) (*models.GPUNode, error) {
		return nil, errors.New("verify instance ocid1.instance.x: not an instance of any GPU pool in this environment")
	}
	d.SoftReset = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		t.Fatal("must not call OCI")
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "node", "node-a", "--ocid", "ocid1.instance.x", "-y"}, "")
	if err == nil || !strings.Contains(err.Error(), "not an instance of any GPU pool") {
		t.Fatalf("expected refusal, got: %v", err)
	}
	if strings.Contains(out, "Confirm") {
		t.Errorf("no prompt for a refused OCID, got: %q", out)
	}
}

func TestRestartNodeCmd_NameResolvesViaCluster(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, _ string) (*models.GPUNode, error) {
		if name != "node-a" {
			t.Errorf("resolver got %q, want node-a", name)
		}
		return &models.GPUNode{Name: name, ID: "ocid1.resolved"}, nil
	}

	var gotNode *models.GPUNode
	d.SoftReset = func(_ context.Context, n *models.GPUNode, _ models.Environment, _ *slog.Logger) error {
		gotNode = n
		return nil
	}

	if _, err := runRootCmdWith(t, d, []string{"restart", "node", "node-a", "-y"}, ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode == nil || gotNode.ID != "ocid1.resolved" {
		t.Errorf("expected resolver-supplied node, got: %+v", gotNode)
	}
}

func TestRestartNodeCmd_ResolverNotFound(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	d.ResolveGPUNode = func(context.Context, models.Environment, string, string) (*models.GPUNode, error) {
		return nil, errors.New("gpu node \"node-missing\" not found in any pool")
	}

	_, err := runRootCmdWith(t, d, []string{"restart", "node", "node-missing", "-y"}, "")
	if err == nil {
		t.Fatal("expected error when node not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}
