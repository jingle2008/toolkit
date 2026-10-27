//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// A dry run looks its target up as a real call would, so a target that
// doesn't exist fails the dry run instead of reporting "would ...".
func TestDryRun_MissingTargetFails(t *testing.T) {
	notFound := errors.New(`nodes "ghost" not found`)
	cases := []struct {
		name string
		args []string
		stub func(d *ops.Deps)
	}{
		{"cordon", []string{"cordon", "ghost", "-n"}, func(d *ops.Deps) {
			d.NodeCordoned = func(context.Context, string, string, string) (bool, error) { return false, notFound }
		}},
		{"uncordon", []string{"uncordon", "ghost", "-n"}, func(d *ops.Deps) {
			d.NodeCordoned = func(context.Context, string, string, string) (bool, error) { return false, notFound }
		}},
		{"drain", []string{"drain", "ghost", "-n"}, func(d *ops.Deps) {
			d.DrainStatus = func(context.Context, string, string, string) (k8s.DrainStatus, error) {
				return k8s.DrainStatus{}, notFound
			}
		}},
		{"delete dac", []string{"delete", "dac", "ghost", "-n"}, func(d *ops.Deps) {
			d.GetDAC = func(context.Context, *models.DedicatedAICluster, models.Environment) (oci.DACState, error) {
				return oci.DACState{}, notFound
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stageMutationEnv(t)
			d := &ops.Deps{}
			tc.stub(d)
			out, err := runRootCmdWith(t, d, tc.args, "")
			require.ErrorIs(t, err, notFound)
			assert.NotContains(t, out, "DRY-RUN")
		})
	}
}

// A dry run reports what the lookup found.
func TestDryRun_ReportsLookup(t *testing.T) {
	d := &ops.Deps{}
	stageMutationEnv(t)
	d.NodeCordoned = func(context.Context, string, string, string) (bool, error) { return true, nil }
	out, err := runRootCmdWith(t, d, []string{"cordon", "node-a", "-n"}, "")
	require.NoError(t, err)
	assert.Contains(t, out, "DRY-RUN: would cordon node/node-a (cordoned=true)")

	out, err = runRootCmdWith(t, d, []string{"drain", "node-a", "-n"}, "")
	require.NoError(t, err)
	assert.Contains(t, out, "DRY-RUN: would drain node/node-a (drain_phase=none)")

	out, err = runRootCmdWith(t, d, []string{"delete", "dac", "dac-x", "-n"}, "")
	require.NoError(t, err)
	assert.Contains(t, out, "dac_id=ocid1.generativeaidedicatedaicluster.oc1.iad.dac-x, state=ACTIVE")
}

// set tenant's dry run makes the write's checks: a name another tenancy
// in the realm already has is refused, as the write would refuse it.
func TestSetTenant_DryRunChecksNameUniqueness(t *testing.T) {
	stageTenantEnv(t)
	meta := filepath.Join(os.Getenv("HOME"), ".config", "toolkit", "metadata.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(meta), 0o700))                                                            // #nosec G703 -- the test's temp HOME
	require.NoError(t, os.WriteFile(meta, []byte("tenants:\n  - id: ocid1.tenancy.oc1..other\n    name: acme\n"), 0o600)) // #nosec G703 -- the test's temp HOME
	// The real check, against that file.
	d := &ops.Deps{CheckTenant: func(e models.TenantMetadata) (bool, error) { return configloader.CheckTenantUpsert(meta, e) }}

	out, err := runRootCmdWith(t, d, []string{"set", "tenant", testTenancyOCID, "--name", "acme", "-n"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tenant name "acme" is already used by ocid1.tenancy.oc1..other`)
	assert.NotContains(t, out, "DRY-RUN")

	out, err = runRootCmdWith(t, d, []string{"set", "tenant", "ocid1.tenancy.oc1..other", "--name", "acme2", "-n"}, "")
	require.NoError(t, err)
	assert.True(t, strings.Contains(out, "(exists=true)"), out)
}
