package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// increasePoolSizeFn is the seam tests use to fake the OCI call.
var increasePoolSizeFn = oci.IncreasePoolSize

// addScaleCommand wires `toolkit scale gpu-pool <name> [--add N]`.
// Without --add it fills the pool to its Terraform-declared size and does
// nothing when it is there, so a script running it after terraform apply
// keeps that meaning. --add N grows the pool by N instances, past the
// declared size if need be; that drift lasts until the next terraform
// apply, which the prompt and dry run warn about.
func addScaleCommand(rootCmd *cobra.Command, cfgFile *string) {
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

Without --add, fills the pool to its Terraform-declared size, and does
nothing when it is already there. --add N adds N instances to the size
OCI reports at the time, which may take the pool past its declared
size: the next terraform apply shrinks it back and terminates the extra
instances, so raise size in Terraform to keep them.

Fire-and-forget; the work request can be tracked via the OCI console.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			addGiven := cmd.Flags().Changed("add")
			return withMutationSetup(cfgFile, true, true, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				var (
					pool   *models.GPUPool
					n      int
					result oci.PoolScale
					done   bool
				)
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action: mutation.ScaleGPUPool,
					Target: name,
					DryRun: dryRun,
					Yes:    yes,
					Resolve: func(ctx context.Context, p *mutationPlan) error {
						found, err := resolveGPUPoolFn(ctx, cfg, env, name)
						if err != nil {
							return err
						}
						if found.ID == "" {
							return fmt.Errorf("pool %s: %w", found.Name, oci.ErrNoInstancePool)
						}
						if n, err = mutation.PoolScaleAdd(*found, add, addGiven); err != nil {
							return err
						}
						if n == 0 {
							p.Skip = mutation.PoolAtSizeNote(*found, "--add")
							return nil
						}
						pool, p.Resolved = found, mutation.ResolvedPool(found.ID, found.Size, found.ActualSize, n)
						p.Notice = mutation.PoolScaleNotice(*found, n)
						return nil
					},
					AuditFields: func() []any {
						if !done {
							return []any{"add", n}
						}
						return []any{"add", n, "from", result.Before, "to", result.After}
					},
					Outcome: func() string { return result.String() }, //nolint:gocritic // perform fills result in later; a method value would bind it now

				}, func(ctx context.Context) error {
					var err error
					result, err = increasePoolSizeFn(ctx, pool, n, env, logging.FromContext(ctx))
					done = err == nil
					return err
				})
			})
		},
	}
	gpuPoolCmd.Flags().IntVar(&add, "add", 0, "Instances to add, counted from OCI's current size; may go past the Terraform size (default: fill to the Terraform size)")
	gpuPoolCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	gpuPoolCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")

	scaleCmd.AddCommand(gpuPoolCmd)
	rootCmd.AddCommand(scaleCmd)
}
