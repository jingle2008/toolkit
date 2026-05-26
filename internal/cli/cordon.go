package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/pkg/models"
)

// setCordonFn is the seam tests use to fake the k8s call. Production
// callers go through k8s.SetCordon, which reaches a live cluster.
var setCordonFn = k8s.SetCordon

func addCordonCommand(rootCmd *cobra.Command, cfgFile *string) {
	addCordonOrUncordon(rootCmd, cfgFile, "cordon", true,
		"Mark a node unschedulable (idempotent)")
}

func addUncordonCommand(rootCmd *cobra.Command, cfgFile *string) {
	addCordonOrUncordon(rootCmd, cfgFile, "uncordon", false,
		"Mark a node schedulable (idempotent)")
}

// addCordonOrUncordon wires either subcommand. want=true → cordon
// (Unschedulable=true), want=false → uncordon. Both go through the
// idempotent k8s.SetCordon path.
func addCordonOrUncordon(rootCmd *cobra.Command, cfgFile *string, verb string, want bool, short string) {
	var (
		dryRun bool
		yes    bool
	)
	long := `Mark a Kubernetes node unschedulable. Existing pods stay
where they are; new pods skip the node.

The operation is idempotent: an already-cordoned node returns
"already cordoned" with no error. Pair with ` + "`toolkit drain`" + ` if
you want to actually evict the existing pods.

Examples:
  toolkit cordon gpu-node-42 --dry-run
  toolkit cordon gpu-node-42 -y
  toolkit cordon gpu-node-42                # interactive confirm`
	if !want {
		long = `Mark a Kubernetes node schedulable again. New pods can land
on the node; existing assignments are unaffected.

The operation is idempotent: an already-uncordoned node returns
"already uncordoned" with no error.

Examples:
  toolkit uncordon gpu-node-42 --dry-run
  toolkit uncordon gpu-node-42 -y
  toolkit uncordon gpu-node-42              # interactive confirm`
	}
	cmd := &cobra.Command{
		Use:   verb + " <node>",
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]
			return withMutationSetup(cfgFile, true, false, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				out := cmd.OutOrStdout()
				return runMutation(ctx, cmd.InOrStdin(), out, mutationPlan{
					Action:  verb,
					Kind:    "node",
					Target:  nodeName,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					changed, err := setCordonFn(ctx, cfg.KubeConfig, env.KubeContext(), nodeName, want)
					if err != nil {
						return err
					}
					if !changed {
						_, _ = fmt.Fprintf(out, "note: node already %sed; no change made\n", verb)
					}
					return nil
				})
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	rootCmd.AddCommand(cmd)
}
