//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDrainCmd_DryRun_DoesNotCallK8s(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := drainNodeFn
	defer func() { drainNodeFn = orig }()
	drainNodeFn = func(context.Context, string, string, string) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call k8s")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would drain node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestDrainCmd_InteractiveBail(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := drainNodeFn
	defer func() { drainNodeFn = orig }()
	drainNodeFn = func(context.Context, string, string, string) error {
		called = true
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a"})
	cmd.SetIn(strings.NewReader("n\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("must not call k8s after user types n")
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Errorf("expected 'aborted', got: %q", out.String())
	}
}

func TestDrainCmd_YesCallsK8s(t *testing.T) {
	stageMutationEnv(t)
	var gotNode string
	orig := drainNodeFn
	defer func() { drainNodeFn = orig }()
	drainNodeFn = func(_ context.Context, _, _, node string) error {
		gotNode = node
		return nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode != "node-a" {
		t.Errorf("node: got %q", gotNode)
	}
	if !strings.Contains(out.String(), "drain node/node-a: OK") {
		t.Errorf("expected OK, got: %q", out.String())
	}
}

func TestDrainCmd_PerformError(t *testing.T) {
	stageMutationEnv(t)
	orig := drainNodeFn
	defer func() { drainNodeFn = orig }()
	drainNodeFn = func(context.Context, string, string, string) error {
		return errors.New("pods stuck terminating")
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error to surface")
	}
	if !strings.Contains(err.Error(), "pods stuck terminating") {
		t.Errorf("error must wrap underlying message: %v", err)
	}
}
