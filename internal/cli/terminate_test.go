//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTerminateCmd_DryRun_DoesNotCallOCI(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	called := false
	d.Terminate = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		called = true
		return nil
	}

	// Note: dry-run must work without --yes — it only previews.
	out, err := runRootCmdWith(t, d, []string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out, "DRY-RUN: would terminate node/ocid1.instance.fake (gpu_pool=pool-a, instance_id=ocid1.instance.fake)") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestTerminateCmd_RequiresExplicitYes(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	called := false
	d.Terminate = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		called = true
		return nil
	}

	_, err := runRootCmdWith(t, d, []string{"terminate", "node-a", "--ocid", "ocid1.instance.fake"}, "y\n") // typing y must NOT be enough
	if err == nil {
		t.Fatal("expected error: destructive op requires --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes: %v", err)
	}
	if called {
		t.Fatal("must not call OCI without --yes")
	}
}

func TestTerminateCmd_YesCallsOCI(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	var gotNode *models.GPUNode
	d.Terminate = func(_ context.Context, n *models.GPUNode, _ models.Environment, _ *slog.Logger) error {
		gotNode = n
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode == nil || gotNode.ID != "ocid1.instance.fake" {
		t.Errorf("expected the verified --ocid node, got: %+v", gotNode)
	}
	if !strings.Contains(out, "terminate node/ocid1.instance.fake: OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestTerminateCmd_NameResolvesViaCluster(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, _ string) (*models.GPUNode, error) {
		return &models.GPUNode{Name: name, ID: "ocid1.resolved"}, nil
	}
	var gotNode *models.GPUNode
	d.Terminate = func(_ context.Context, n *models.GPUNode, _ models.Environment, _ *slog.Logger) error {
		gotNode = n
		return nil
	}

	if _, err := runRootCmdWith(t, d, []string{"terminate", "node-a", "--yes"}, ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode == nil || gotNode.ID != "ocid1.resolved" {
		t.Errorf("expected resolver-supplied node, got: %+v", gotNode)
	}
}

func TestTerminateCmd_PerformError(t *testing.T) {
	d := &ops.Deps{}
	stageOCIDEnv(t, d)
	d.Terminate = func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error {
		return errors.New("instance already terminating")
	}

	_, err := runRootCmdWith(t, d, []string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error to surface")
	}
	if !strings.Contains(err.Error(), "instance already terminating") {
		t.Errorf("error must wrap underlying: %v", err)
	}
}
