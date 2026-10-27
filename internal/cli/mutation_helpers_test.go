package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
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

// stageOCIDEnv is stageMutationEnv for a node action given --ocid: it
// adds the repo path that check needs, and stubs d's lookup so the OCID
// is an instance of GPU pool pool-a.
func stageOCIDEnv(t *testing.T, d *ops.Deps) {
	t.Helper()
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir())
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, ocid string) (*models.GPUNode, error) {
		if ocid == "" {
			t.Errorf("node %q resolved without --ocid", name)
		}
		return &models.GPUNode{Name: name, ID: ocid, NodePool: "pool-a"}, nil
	}
}

// testLookups is the read-only target lookups that cordon, drain,
// delete dac and set tenant run before a dry run or prompt, finding
// their target, so no test reaches a cluster, OCI or the metadata file.
// A test that wants a lookup to fail sets its own.
func testLookups() ops.Deps {
	return ops.Deps{
		NodeCordoned: func(context.Context, string, string, string) (bool, error) { return false, nil },
		DrainStatus: func(_ context.Context, _, _, node string) (k8s.DrainStatus, error) {
			return k8s.DrainStatus{Node: node, DrainState: k8s.DrainState{Phase: k8s.DrainNone}}, nil
		},
		GetDAC: func(_ context.Context, d *models.DedicatedAICluster, env models.Environment) (oci.DACState, error) {
			return oci.DACState{ID: d.OCID(env.Realm, env.Region), State: "ACTIVE"}, nil
		},
		CheckTenant: func(models.TenantMetadata) (bool, error) { return false, nil },
	}
}

// fillDeps sets every call d leaves unset to from's.
func fillDeps(d *ops.Deps, from ops.Deps) {
	dv, fv := reflect.ValueOf(d).Elem(), reflect.ValueOf(from)
	for i := range dv.NumField() {
		if dv.Field(i).IsNil() {
			dv.Field(i).Set(fv.Field(i))
		}
	}
}

// testRootCmd is a root command whose mutations' infra calls are d's;
// what d leaves unset (or all, for a nil d) is testLookups', then
// production's.
func testRootCmd(d *ops.Deps) *cobra.Command {
	return newRootCmd("vtest", func(cfg config.Config) ops.Deps {
		var dd ops.Deps
		if d != nil {
			dd = *d
		}
		fillDeps(&dd, testLookups())
		fillDeps(&dd, productionDeps(cfg))
		return dd
	})
}

// runRootCmd is runRootCmdWith with no fakes of its own and no stdin.
func runRootCmd(t *testing.T, args []string) (string, error) {
	t.Helper()
	return runRootCmdWith(t, nil, args, "")
}

// runRootCmdWith builds testRootCmd(d), wires stdin/stdout/stderr to
// in-memory buffers, runs Execute, and returns the combined out+err
// buffer plus the execution error. stdin is wired only when non-empty
// so callers that don't need it inherit the cobra default.
func runRootCmdWith(t *testing.T, d *ops.Deps, args []string, stdin string) (string, error) {
	t.Helper()
	cmd := testRootCmd(d)
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
