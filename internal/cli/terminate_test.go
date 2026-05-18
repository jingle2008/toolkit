//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTerminateCmd_DryRun_DoesNotCallOCI(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := terminateInstanceFn
	defer func() { terminateInstanceFn = orig }()
	terminateInstanceFn = func(context.Context, *models.GpuNode, models.Environment, logging.Logger) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	// Note: dry-run must work without --yes — it only previews.
	cmd.SetArgs([]string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would terminate node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestTerminateCmd_RequiresExplicitYes(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := terminateInstanceFn
	defer func() { terminateInstanceFn = orig }()
	terminateInstanceFn = func(context.Context, *models.GpuNode, models.Environment, logging.Logger) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"terminate", "node-a", "--ocid", "ocid1.instance.fake"})
	cmd.SetIn(strings.NewReader("y\n")) // typing y must NOT be enough
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
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
	stageMutationEnv(t)
	var gotNode *models.GpuNode
	orig := terminateInstanceFn
	defer func() { terminateInstanceFn = orig }()
	terminateInstanceFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--yes"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode == nil || gotNode.ID != "ocid1.instance.fake" {
		t.Errorf("expected synthesized node from --ocid, got: %+v", gotNode)
	}
	if !strings.Contains(out.String(), "terminate node/node-a: OK") {
		t.Errorf("expected OK, got: %q", out.String())
	}
}

func TestTerminateCmd_NameResolvesViaCluster(t *testing.T) {
	stageMutationEnv(t)
	origResolver := gpuNodeResolverFn
	defer func() { gpuNodeResolverFn = origResolver }()
	gpuNodeResolverFn = func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GpuNode, error) {
		return &models.GpuNode{Name: name, ID: "ocid1.resolved"}, nil
	}
	var gotNode *models.GpuNode
	orig := terminateInstanceFn
	defer func() { terminateInstanceFn = orig }()
	terminateInstanceFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"terminate", "node-a", "--yes"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode == nil || gotNode.ID != "ocid1.resolved" {
		t.Errorf("expected resolver-supplied node, got: %+v", gotNode)
	}
}

func TestTerminateCmd_PerformError(t *testing.T) {
	stageMutationEnv(t)
	orig := terminateInstanceFn
	defer func() { terminateInstanceFn = orig }()
	terminateInstanceFn = func(context.Context, *models.GpuNode, models.Environment, logging.Logger) error {
		return errors.New("instance already terminating")
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"terminate", "node-a", "--ocid", "ocid1.instance.fake", "--yes"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error to surface")
	}
	if !strings.Contains(err.Error(), "instance already terminating") {
		t.Errorf("error must wrap underlying: %v", err)
	}
}
