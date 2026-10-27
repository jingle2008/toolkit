//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/mutation/ops"
)

// stubResolveISVC fakes d's cluster lookup: an empty namespace resolves
// to resolvedNS, an explicit one is echoed back.
func stubResolveISVC(t *testing.T, d *ops.Deps, resolvedNS string, gotNS *string) {
	t.Helper()
	d.ResolveISVC = func(_ context.Context, _, _, namespace, name string) (string, error) {
		*gotNS = namespace
		if name != "isvc-a" {
			t.Errorf("resolver got name %q, want isvc-a", name)
		}
		if namespace != "" {
			return namespace, nil
		}
		return resolvedNS, nil
	}
}

func TestRestartCmd_DryRun_ResolvesButDoesNotPatch(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	var gotNS string
	stubResolveISVC(t, d, "ns-a", &gotNS)
	called := false
	d.RestartISVC = func(context.Context, string, string, string, string) error {
		called = true
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "isvc", "isvc-a", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not patch the InferenceService")
	}
	if gotNS != "" {
		t.Errorf("resolver should search all namespaces without --namespace, got %q", gotNS)
	}
	if !strings.Contains(out, "DRY-RUN: would restart inference_service/ns-a/isvc-a") {
		t.Errorf("expected DRY-RUN line with resolved namespace, got: %q", out)
	}
}

func TestRestartCmd_YesPatchesResolvedTarget(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	var gotNS string
	stubResolveISVC(t, d, "ns-a", &gotNS)
	var patchedNS, patchedName, kubeCtx string
	d.RestartISVC = func(_ context.Context, _, contextName, namespace, name string) error {
		kubeCtx, patchedNS, patchedName = contextName, namespace, name
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "inference-service", "isvc-a", "--namespace", "ns-explicit", "-y"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotNS != "ns-explicit" {
		t.Errorf("resolver should receive --namespace, got %q", gotNS)
	}
	if patchedNS != "ns-explicit" || patchedName != "isvc-a" {
		t.Errorf("patched %s/%s, want ns-explicit/isvc-a", patchedNS, patchedName)
	}
	if kubeCtx == "" {
		t.Error("expected env-derived kube context")
	}
	if !strings.Contains(out, "restart inference_service/ns-explicit/isvc-a: OK") {
		t.Errorf("expected OK line, got: %q", out)
	}
}

func TestRestartCmd_InteractivePrompt(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	var gotNS string
	stubResolveISVC(t, d, "ns-a", &gotNS)
	called := false
	d.RestartISVC = func(context.Context, string, string, string, string) error {
		called = true
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"restart", "isvc", "isvc-a"}, "n\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called || !strings.Contains(out, "aborted") {
		t.Errorf("answering n must abort without patching; called=%v out=%q", called, out)
	}

	out, err = runRootCmdWith(t, d, []string{"restart", "isvc", "isvc-a"}, "y\n")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !called {
		t.Errorf("answering y must patch; out=%q", out)
	}
}

func TestRestartCmd_ResolverErrorStopsBeforePrompt(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	d.ResolveISVC = func(context.Context, string, string, string, string) (string, error) {
		return "", errors.New(`InferenceService "isvc-a" exists in multiple namespaces (a, b); specify a namespace`)
	}
	d.RestartISVC = func(context.Context, string, string, string, string) error {
		t.Error("restart must not run when resolution fails")
		return nil
	}

	_, err := runRootCmdWith(t, d, []string{"restart", "isvc", "isvc-a", "-y"}, "")
	if err == nil || !strings.Contains(err.Error(), "multiple namespaces") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}
