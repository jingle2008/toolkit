//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// stageMutationEnv writes a fake kubeconfig to a tempdir, sets HOME to
// that dir, and exports the env triple via TOOLKIT_* so a fresh viper
// state passes validateMutationConfig.
func stageMutationEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	kc := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(kc, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("TOOLKIT_KUBECONFIG", kc)
	t.Setenv("TOOLKIT_ENV_TYPE", "dev")
	t.Setenv("TOOLKIT_ENV_REGION", "us-ashburn-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc1")
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestCordonCmd_DryRun_DoesNotCallK8s(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		called = true
		return true, nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call k8s")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would cordon node/node-a") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestCordonCmd_InteractiveBail(t *testing.T) {
	stageMutationEnv(t)
	called := false
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		called = true
		return true, nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a"})
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

func TestCordonCmd_YesSkipsPromptAndCalls(t *testing.T) {
	stageMutationEnv(t)
	var (
		gotKube    string
		gotContext string
		gotNode    string
		gotWant    bool
	)
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(_ context.Context, kc, ctxName, node string, want bool) (bool, error) {
		gotKube, gotContext, gotNode, gotWant = kc, ctxName, node, want
		return true, nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
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
	if !strings.Contains(out.String(), "cordon node/node-a: OK") {
		t.Errorf("expected OK, got: %q", out.String())
	}
}

func TestUncordonCmd_PassesWantFalse(t *testing.T) {
	stageMutationEnv(t)
	var gotWant bool
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(_ context.Context, _, _, _ string, want bool) (bool, error) {
		gotWant = want
		return true, nil
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"uncordon", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotWant {
		t.Error("uncordon must call SetCordon with want=false")
	}
}

func TestCordonCmd_NoChangeNote(t *testing.T) {
	stageMutationEnv(t)
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		return false, nil // already in target state
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "already cordoned") {
		t.Errorf("expected no-change note, got: %q", out.String())
	}
}

func TestCordonCmd_PerformError(t *testing.T) {
	stageMutationEnv(t)
	orig := setCordonFn
	defer func() { setCordonFn = orig }()
	setCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		return false, errors.New("kube unreachable")
	}

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
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

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"cordon", "node-a", "-y"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when kubeconfig missing")
	}
	if !strings.Contains(err.Error(), "kubeconfig") {
		t.Errorf("error should mention kubeconfig: %v", err)
	}
}
