//nolint:paralleltest // NewRootCmd uses cobra/viper global state
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// runTenantSync runs `toolkit --config <cfg> tenant sync` with the given
// config file contents and returns the command's error.
func runTenantSync(t *testing.T, cfg string) error {
	t.Helper()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o600))
	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "tenant", "sync"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd.Execute()
}

func TestTenantSyncCmd_NeedsRepoPath(t *testing.T) {
	require.ErrorContains(t, runTenantSync(t, "env-realm: oc1\n"), "no repo path")
}

// repo-path and metadata-file come from toolkit's config; an invalid
// metadata file stops the sync before any lookup.
func TestTenantSyncCmd_UsesConfigPaths(t *testing.T) {
	dir := t.TempDir()
	meta := filepath.Join(dir, "metadata.yaml")
	require.NoError(t, os.WriteFile(meta, []byte("tenants:\n- {id: a, name: x}\n- {id: b, name: x}\n"), 0o600))
	err := runTenantSync(t, "repo-path: "+dir+"\nmetadata-file: "+meta+"\n")
	require.ErrorContains(t, err, "invalid tenant metadata")
}
