package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// increasePoolSizeFn is the seam tests use to fake the OCI call.
var increasePoolSizeFn = actions.IncreasePoolSize

// addScaleCommand wires `toolkit scale gpupool <name>`. Terraform is
// the source of truth for size: the action reads pool.Size (loaded
// from the IaC repo) and submits an UpdateInstancePool to match it.
// No --size flag — call this after `terraform apply` to push the
// IaC-declared size to OCI. Adding API-level size override would
// invite drift from Terraform; deliberately omitted.
func addScaleCommand(rootCmd *cobra.Command, cfgFile *string) {
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Sync IaC-declared size to OCI.",
	}

	var (
		dryRun bool
		yes    bool
	)
	gpuPoolCmd := &cobra.Command{
		Use:   "gpupool <name>",
		Short: "Sync a GPU pool's OCI instance-pool size to the Terraform-declared size.",
		Long: `Reads pool.Size from the Terraform repo and submits an
UpdateInstancePool request to OCI for the matching live instance pool.
No --size flag: Terraform is the source of truth, and we deliberately
avoid letting CLI calls drift from IaC.

Fire-and-forget; the work request can be tracked via the OCI console.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withMutationSetup(cfgFile, true, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "scale",
					Kind:    "gpu_pool",
					Target:  name,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					pool, err := gpuPoolResolverFn(ctx, cfg, env, name)
					if err != nil {
						return err
					}
					return increasePoolSizeFn(ctx, pool, env, logging.FromContext(ctx))
				})
			})
		},
	}
	gpuPoolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	gpuPoolCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")

	scaleCmd.AddCommand(gpuPoolCmd)
	rootCmd.AddCommand(scaleCmd)
}
