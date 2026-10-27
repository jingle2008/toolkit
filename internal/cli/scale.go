package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// addScaleCommand wires `toolkit scale gpu-pool <name> [--add N]`.
// Without --add it fills the pool to its Terraform-declared size and does
// nothing when it is there, so a script running it after terraform apply
// keeps that meaning. --add N grows the pool by N instances, past the
// declared size if need be; that drift lasts until the next terraform
// apply, which the prompt and dry run warn about.
func addScaleCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	scaleCmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale up OCI capacity",
	}

	var (
		dryRun bool
		yes    bool
		add    int
	)
	gpuPoolCmd := &cobra.Command{
		Use:     "gpu-pool <name>",
		Aliases: []string{"gpupool"},
		Short:   "Grow a GPU pool's OCI instance pool",
		Long: `Grows the OCI instance pool of a GPU pool from the Terraform repo.

Without --add, fills the pool to its Terraform-declared size, counted
from the size OCI reports at the time, and does nothing when it is
already there. --add N adds N instances to the size
OCI reports at the time, which may take the pool past its declared
size: the next terraform apply shrinks it back and terminates the extra
instances, so raise size in Terraform to keep them.

Fire-and-forget; the work request can be tracked via the OCI console.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			addGiven := cmd.Flags().Changed("add")
			return withMutationSetup(cmd.Context(), cfgFile, true, true, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				op, scale := ops.ScalePool(deps(cfg), opsEnv(cfg, env), name, add, addGiven, "--add")
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cliGate{DryRun: dryRun, Yes: yes, Outcome: scale.Sizes}, op)
			})
		},
	}
	gpuPoolCmd.Flags().IntVar(&add, "add", 0, "Instances to add, counted from OCI's current size; may go past the Terraform size (default: fill to the Terraform size)")
	gpuPoolCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	gpuPoolCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")

	scaleCmd.AddCommand(gpuPoolCmd)
	rootCmd.AddCommand(scaleCmd)
}
