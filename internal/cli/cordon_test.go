//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCordonCmd_DryRun_DoesNotCallK8s(t *testing.T) {
	stageMutationEnv(t)
	called := false
	defer swap(&setCordonFn, func(context.Context, string, string, string, bool) (bool, error) {
		called = true
		return true, nil
	})()

	out, err := runRootCmd(t, []string{"cordon", "node-a", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call k8s")
	}
	if !strings.Contains(out, "DRY-RUN: would cordon node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestCordonCmd_InteractiveBail(t *testing.T) {
	stageMutationEnv(t)
	called := false
	defer swap(&setCordonFn, func(context.Context, string, string, string, bool) (bool, error) {
		called = true
		return true, nil
	})()

	out, err := runRootCmd(t, []string{"cordon", "node-a"}, "n\n")
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

func TestCordonCmd_YesSkipsPromptAndCalls(t *testing.T) {
	stageMutationEnv(t)
	var (
		gotKube    string
		gotContext string
		gotNode    string
		gotWant    bool
	)
	defer swap(&setCordonFn, func(_ context.Context, kc, ctxName, node string, want bool) (bool, error) {
		gotKube, gotContext, gotNode, gotWant = kc, ctxName, node, want
		return true, nil
	})()

	out, err := runRootCmd(t, []string{"cordon", "node-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNode != "node-a" {
		t.Errorf("node: got %q", gotNode)
	}
	if !gotWant {
		t.Error("cordon must call SetCordon with want=true")
	}
	if gotKube == "" {
		t.Error("kubeconfig should be set from env")
	}
	if !strings.HasPrefix(gotContext, "dp-") {
		t.Errorf("kube context should be derived (dp-…), got %q", gotContext)
	}
	if !strings.Contains(out, "cordon node/node-a: OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestUncordonCmd_PassesWantFalse(t *testing.T) {
	stageMutationEnv(t)
	var gotWant bool
	defer swap(&setCordonFn, func(_ context.Context, _, _, _ string, want bool) (bool, error) {
		gotWant = want
		return true, nil
	})()

	if _, err := runRootCmd(t, []string{"uncordon", "node-a", "-y"}, ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotWant {
		t.Error("uncordon must call SetCordon with want=false")
	}
}

func TestCordonCmd_NoChangeNote(t *testing.T) {
	stageMutationEnv(t)
	defer swap(&setCordonFn, func(context.Context, string, string, string, bool) (bool, error) {
		return false, nil // already in target state
	})()

	out, err := runRootCmd(t, []string{"cordon", "node-a", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "already cordoned") {
		t.Errorf("expected no-change note, got: %q", out)
	}
}

func TestCordonCmd_PerformError(t *testing.T) {
	stageMutationEnv(t)
	defer swap(&setCordonFn, func(context.Context, string, string, string, bool) (bool, error) {
		return false, errors.New("kube unreachable")
	})()

	_, err := runRootCmd(t, []string{"cordon", "node-a", "-y"}, "")
	if err == nil {
		t.Fatal("expected error to surface from cmd.Execute")
	}
	if !strings.Contains(err.Error(), "kube unreachable") {
		t.Errorf("error must wrap underlying message: %v", err)
	}
}

func TestCordonCmd_MissingKubeConfig(t *testing.T) {
	// HOME → tempdir but DON'T export TOOLKIT_KUBECONFIG so the default
	// path resolves to nonexistent ~/.kube/config.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TOOLKIT_ENV_TYPE", "dev")
	t.Setenv("TOOLKIT_ENV_REGION", "us-ashburn-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc1")
	viper.Reset()
	t.Cleanup(viper.Reset)

	_, err := runRootCmd(t, []string{"cordon", "node-a", "-y"}, "")
	if err == nil {
		t.Fatal("expected error when kubeconfig missing")
	}
	if !strings.Contains(err.Error(), "kubeconfig") {
		t.Errorf("error should mention kubeconfig: %v", err)
	}
}
