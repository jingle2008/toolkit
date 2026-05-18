//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDrainCmd_DryRun_DoesNotCallK8s(t *testing.T) {
	stageMutationEnv(t)
	called := false
	defer swap(&drainNodeFn, func(context.Context, string, string, string) error {
		called = true
		return nil
	})()

	out, err := runRootCmd(t, []string{"drain", "node-a", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call k8s")
	}
	if !strings.Contains(out, "DRY-RUN: would drain node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestDrainCmd_InteractiveBail(t *testing.T) {
	stageMutationEnv(t)
	called := false
	defer swap(&drainNodeFn, func(context.Context, string, string, string) error {
		called = true
		return nil
	})()

	out, err := runRootCmd(t, []string{"drain", "node-a"}, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("must not call k8s after user types n")
	}
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted', got: %q", out)
	}
}

func TestDrainCmd_YesCallsK8s(t *testing.T) {
	stageMutationEnv(t)
	var gotNode string
	defer swap(&drainNodeFn, func(_ context.Context, _, _, node string) error {
		gotNode = node
		return nil
	})()

	out, err := runRootCmd(t, []string{"drain", "node-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode != "node-a" {
		t.Errorf("node: got %q", gotNode)
	}
	if !strings.Contains(out, "drain node/node-a: OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestDrainCmd_PerformError(t *testing.T) {
	stageMutationEnv(t)
	defer swap(&drainNodeFn, func(context.Context, string, string, string) error {
		return errors.New("pods stuck terminating")
	})()

	_, err := runRootCmd(t, []string{"drain", "node-a", "-y"}, "")
	if err == nil {
		t.Fatal("expected error to surface")
	}
	if !strings.Contains(err.Error(), "pods stuck terminating") {
		t.Errorf("error must wrap underlying message: %v", err)
	}
}
