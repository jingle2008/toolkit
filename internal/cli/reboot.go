package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// softResetInstanceFn is the seam tests use to fake the OCI call.
var softResetInstanceFn = actions.SoftResetInstance

func addRebootCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		dryRun bool
		yes    bool
		ocid   string
	)
	cmd := &cobra.Command{
		Use:   "reboot <node>",
		Short: "Soft-reset a GPU node's underlying OCI instance.",
		Long: `Submits a soft-reset (graceful reboot) request to OCI for the
instance backing <node>. Fire-and-forget: the call returns as soon as
OCI accepts the request; check status via the OCI console or any
wait-flow tooling.

By default <node> is resolved against the live cluster (kube config +
env triple required). Pass --ocid to skip the cluster lookup when you
already know the instance OCID.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			needsKube := ocid == ""
			return withMutationSetup(cfgFile, needsKube, false, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "reboot",
					Kind:    "node",
					Target:  name,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					node, err := resolveGpuNode(ctx, cfg, env, name, ocid)
					if err != nil {
						return err
					}
					return softResetInstanceFn(ctx, node, env, logging.FromContext(ctx))
				})
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	cmd.Flags().StringVar(&ocid, "ocid", "", "Skip k8s lookup and target this instance OCID directly")
	rootCmd.AddCommand(cmd)
}
