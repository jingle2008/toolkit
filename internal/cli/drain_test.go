//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
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

func TestDrainCmd_EchoesProgressOnStderr(t *testing.T) {
	stageMutationEnv(t)
	defer swap(&drainNodeFn, func(ctx context.Context, _, _, node string) error {
		k8s.DrainProgressFrom(ctx)("drain: started, node cordoned node=" + node)
		return nil
	})()

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"drain", "node-a", "-y"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stderr.String(), "drain: started, node cordoned node=node-a") {
		t.Errorf("expected progress on stderr, got %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "drain: started") {
		t.Errorf("progress leaked onto stdout: %q", stdout.String())
	}
}

// --status prints the recorded drain and drains nothing.
func TestDrainCmd_Status(t *testing.T) {
	stageMutationEnv(t)
	defer swap(&drainNodeFn, func(context.Context, string, string, string) error {
		t.Fatal("--status must not drain")
		return nil
	})()
	defer swap(&drainStatusFn, func(_ context.Context, _, _, node string) (k8s.DrainStatus, error) {
		return k8s.DrainStatus{Node: node, Remaining: []string{"ns/isvc-a"}}, nil
	})()

	out, err := runRootCmd(t, []string{"drain", "node-a", "--status", "-o", "json"}, "")
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got), out)
	assert.Equal(t, "node-a", got["node"])
	assert.Equal(t, []any{"ns/isvc-a"}, got["remaining"])

	out, err = runRootCmd(t, []string{"drain", "node-a", "--status"}, "")
	require.NoError(t, err)
	assert.Contains(t, out, "node: node-a", "yaml by default")
}

func TestDrainCmd_StatusFlagErrors(t *testing.T) {
	stageMutationEnv(t)
	_, err := runRootCmd(t, []string{"drain", "node-a", "-o", "json"}, "")
	require.ErrorContains(t, err, "-o applies only with --status")
	_, err = runRootCmd(t, []string{"drain", "node-a", "--status", "-n"}, "")
	require.ErrorContains(t, err, "--dry-run doesn't apply")
	_, err = runRootCmd(t, []string{"drain", "node-a", "--status", "-o", "table"}, "")
	require.ErrorContains(t, err, "--status prints yaml or json")
}
