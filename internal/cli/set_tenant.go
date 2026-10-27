package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// setTenantFn is the seam tests use to fake the metadata write.
// Production builds a fresh loader and upserts via the optional
// TenantMetadataWriter capability (same path the TUI uses).
var setTenantFn = func(ctx context.Context, cfg config.Config, entry models.TenantMetadata) error {
	ld := production.New(ctx, cfg.MetadataFile)
	writer, ok := ld.(loader.TenantMetadataWriter)
	if !ok {
		return errors.New("loader does not support writing metadata")
	}
	return writer.UpsertTenantMetadata(entry)
}

// checkTenantFn is the seam tests use to fake set tenant's lookup: what
// the write would find, without writing.
var checkTenantFn = func(cfg config.Config, entry models.TenantMetadata) (bool, error) {
	if cfg.MetadataFile == "" {
		return false, errors.New("no metadata file configured")
	}
	return configloader.CheckTenantUpsert(cfg.MetadataFile, entry)
}

func addSetCommand(rootCmd *cobra.Command, cfgFile *string) {
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a resource",
	}

	var (
		name     string
		internal bool
		note     string
		dryRun   bool
		yes      bool
	)
	tenantCmd := &cobra.Command{
		Use:   "tenant <ocid>",
		Short: "Set tenant metadata (name / internal flag / note) by tenancy OCID",
		Long: `Create or replace the metadata entry for a tenancy OCID in the
metadata file (created if absent). This is the headless equivalent of the
TUI's "edit tenant" form. The entry is keyed by the full tenancy OCID and
stored globally, so no --env flags are required.

<ocid> must be a full tenancy OCID (starts with ` + "`ocid1.tenancy.`" + `).

Examples:
  toolkit set tenant ocid1.tenancy.oc1..aaaa --name "Acme Corp" --yes
  toolkit set tenant ocid1.tenancy.oc1..aaaa --name Acme --internal=false --note vip -y`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ocid := args[0]
			if strings.TrimSpace(name) == "" {
				return errors.New("--name is required")
			}
			// Without --internal, keep the recorded flag (e.g. one
			// `tenant sync` set); a new tenant then defaults to internal.
			var isInternal *bool
			if cmd.Flags().Changed("internal") {
				isInternal = &internal
			}
			entry, err := ops.TenantEntry(ocid, name, isInternal, note)
			if err != nil {
				return err
			}
			return withMutationSetup(cmd.Context(), cfgFile, false, false, false, func(ctx context.Context, cfg config.Config, _ models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cliGate{DryRun: dryRun, Yes: yes},
					ops.SetTenant(cliDeps(cfg), entry))
			})
		},
	}
	tenantCmd.Flags().StringVar(&name, "name", "", "Friendly tenant name (required)")
	tenantCmd.Flags().BoolVar(&internal, "internal", true, "Mark the tenant internal (--internal=false for external); omitted keeps the recorded flag, new tenants default to internal")
	tenantCmd.Flags().StringVar(&note, "note", "", "Optional free-form note")
	tenantCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Print what would happen and exit")
	tenantCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")

	setCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(setCmd)
}
