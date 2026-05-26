package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/pkg/models"
)

// drainNodeFn is the seam tests use to fake the k8s call.
var drainNodeFn = k8s.DrainNode

func addDrainCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "drain <node>",
		Short: "Drain pods from a node (cordons first, then evicts)",
		Long: `Drain evicts pods from <node> using the same default kubectl
behavior the TUI uses: IgnoreAllDaemonSets, DeleteEmptyDirData, and
the pod's termination grace period. Use this before terminating a
node so workloads relocate cleanly.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]
			return withMutationSetup(cfgFile, true, false, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "drain",
					Kind:    "node",
					Target:  nodeName,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					return drainNodeFn(ctx, cfg.KubeConfig, env.KubeContext(), nodeName)
				})
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	rootCmd.AddCommand(cmd)
}
