package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/tenantsync"
)

// addTenantCommand registers `toolkit tenant`, the tenant metadata
// commands.
func addTenantCommand(rootCmd *cobra.Command, cfgFile *string) {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage the tenant metadata file",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newTenantSyncCommand(cfgFile))
	rootCmd.AddCommand(cmd)
}

func newTenantSyncCommand(cfgFile *string) *cobra.Command {
	var (
		cacheDir string
		parallel int
		write    bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fill in oc1 tenant names, internal flags and status from the account admin service",
		Long: `Look up every oc1 tenancy in the repo's tenancy overrides and the tenant
metadata file with the oci-ops CLI (account-admin-tool get-account-details),
and record the result in the metadata file:

  - status (ACTIVE, DELETING, TERMINATED, ALWAYS_FREE_ONLY) is refreshed
    on every run;
  - a name or internal flag is only filled in when an entry has none.
    Yours are never overwritten: where the service disagrees, or its name
    is already used by another tenancy, the report lists it;
  - a failed lookup leaves the entry as it was. An expired oci-ops session
    stops the run: re-authenticate (run any oci-ops command and accept the
    prompt), then rerun.

It is a dry run unless --write, which backs the file up first. The
lookups are applied to the file as it is when written, so tenant edits
made meanwhile (set tenant, a TUI or MCP server) are kept. A running TUI
or MCP server shows the synced names and statuses once restarted.

Requires the oci-ops CLI on PATH. oc1 only.

Examples:
  toolkit tenant sync
  toolkit tenant sync --write
  toolkit tenant sync --cache-dir /tmp/tenant-sync --write`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("unmarshal config: %w", err)
			}
			return tenantsync.Run(c.Context(), tenantsync.Config{
				RepoPath:     cfg.RepoPath,
				MetadataFile: cfg.MetadataFile,
				CacheDir:     cacheDir,
				Parallel:     parallel,
				Write:        write,
			}, c.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "",
		"reuse and save raw lookups here, so an interrupted sync resumes (cached statuses are as old as the files)")
	cmd.Flags().IntVar(&parallel, "parallel", 3, "concurrent lookups")
	cmd.Flags().BoolVar(&write, "write", false, "write the metadata file (default: dry run)")
	return cmd
}
