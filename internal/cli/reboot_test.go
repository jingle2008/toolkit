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

func TestRebootCmd_DryRun_DoesNotCallOCI(t *testing.T) {
	stageMutationEnv(t)
	called := false
	origReset := softResetInstanceFn
	defer func() { softResetInstanceFn = origReset }()
	softResetInstanceFn = func(context.Context, *models.GpuNode, models.Environment, logging.Logger) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"reboot", "node-a", "--ocid", "ocid1.instance.fake", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would reboot node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestRebootCmd_OcidBypassesResolver(t *testing.T) {
	// Stage env WITHOUT a kubeconfig — the --ocid path must not need
	// it. Only env triple is required.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TOOLKIT_ENV_TYPE", "dev")
	t.Setenv("TOOLKIT_ENV_REGION", "us-ashburn-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc1")
	viper.Reset()
	t.Cleanup(viper.Reset)

	resolverCalled := false
	origResolver := gpuNodeResolverFn
	defer func() { gpuNodeResolverFn = origResolver }()
	gpuNodeResolverFn = func(context.Context, config.Config, models.Environment, string) (*models.GpuNode, error) {
		resolverCalled = true
		return nil, errors.New("should not be called")
	}

	var gotNode *models.GpuNode
	origReset := softResetInstanceFn
	defer func() { softResetInstanceFn = origReset }()
	softResetInstanceFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"reboot", "node-a", "--ocid", "ocid1.instance.fake", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resolverCalled {
		t.Error("--ocid must bypass the cluster resolver")
	}
	if gotNode == nil || gotNode.ID != "ocid1.instance.fake" || gotNode.Name != "node-a" {
		t.Errorf("unexpected synthesized node: %+v", gotNode)
	}
}

func TestRebootCmd_NameResolvesViaCluster(t *testing.T) {
	stageMutationEnv(t)
	origResolver := gpuNodeResolverFn
	defer func() { gpuNodeResolverFn = origResolver }()
	gpuNodeResolverFn = func(_ context.Context, _ config.Config, _ models.Environment, name string) (*models.GpuNode, error) {
		if name != "node-a" {
			t.Errorf("resolver got %q, want node-a", name)
		}
		return &models.GpuNode{Name: name, ID: "ocid1.resolved"}, nil
	}

	var gotNode *models.GpuNode
	origReset := softResetInstanceFn
	defer func() { softResetInstanceFn = origReset }()
	softResetInstanceFn = func(_ context.Context, n *models.GpuNode, _ models.Environment, _ logging.Logger) error {
		gotNode = n
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"reboot", "node-a", "-y"})
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

func TestRebootCmd_ResolverNotFound(t *testing.T) {
	stageMutationEnv(t)
	origResolver := gpuNodeResolverFn
	defer func() { gpuNodeResolverFn = origResolver }()
	gpuNodeResolverFn = func(context.Context, config.Config, models.Environment, string) (*models.GpuNode, error) {
		return nil, errors.New("gpu node \"node-missing\" not found in any pool")
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"reboot", "node-missing", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when node not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}
