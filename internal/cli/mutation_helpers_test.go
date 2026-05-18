//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"bytes"
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

// runRootCmd builds a fresh root command, wires stdin/stdout/stderr to
// in-memory buffers, runs Execute, and returns the combined out+err
// buffer plus the execution error. stdin is wired only when non-empty
// so callers that don't need it inherit the cobra default.
func runRootCmd(t *testing.T, args []string, stdin string) (string, error) {
	t.Helper()
	cmd := NewRootCmd("vtest")
	cmd.SetArgs(args)
	if stdin != "" {
		cmd.SetIn(strings.NewReader(stdin))
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

// swap replaces *dst with replacement and returns a closure that
// restores the original. Idiomatic use is `defer swap(&seamFn, mock)()`
// — the trailing call invokes the swap immediately and the returned
// restore closure runs at scope exit. Compact replacement for the
// repeated orig := X; defer func(){ X = orig }(); X = mock pattern.
func swap[F any](dst *F, replacement F) func() {
	orig := *dst
	*dst = replacement
	return func() { *dst = orig }
}
