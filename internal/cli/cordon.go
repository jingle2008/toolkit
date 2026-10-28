package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

func addCordonCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	addCordonOrUncordon(rootCmd, cfgFile, deps, "cordon", true,
		"Mark a node unschedulable (idempotent)")
}

func addUncordonCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	addCordonOrUncordon(rootCmd, cfgFile, deps, "uncordon", false,
		"Mark a node schedulable (idempotent)")
}

// addCordonOrUncordon wires either subcommand. unschedulable=true →
// cordon (k8s Unschedulable=true), unschedulable=false → uncordon.
// Both go through the idempotent k8s.SetCordon path.
func addCordonOrUncordon(rootCmd *cobra.Command, cfgFile *string, deps depsFor, verb string, unschedulable bool, short string) {
	var gate cliGate
	long := `Mark a Kubernetes node unschedulable. Existing pods stay
where they are; new pods skip the node.

The operation is idempotent: an already-cordoned node returns
"already cordoned" with no error. Pair with ` + "`toolkit drain`" + ` if
you want to actually evict the existing pods.

Examples:
  toolkit cordon gpu-node-42 --dry-run
  toolkit cordon gpu-node-42 -y
  toolkit cordon gpu-node-42                # interactive confirm`
	if !unschedulable {
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
			return withMutationSetup(cmd, cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				out := cmd.OutOrStdout()
				op, changed := ops.Cordon(deps(cfg), opsEnv(cfg, env), nodeName, unschedulable)
				perform := op.Perform
				op.Perform = func(ctx context.Context, p mutation.Plan) error {
					if err := perform(ctx, p); err != nil {
						return err
					}
					if !*changed {
						_, _ = fmt.Fprintf(out, "note: node already %sed; no change made\n", verb)
					}
					return nil
				}
				return runMutation(ctx, cmd.InOrStdin(), out, gate, op)
			})
		},
	}
	gateAction := mutation.CordonNode
	if !unschedulable {
		gateAction = mutation.UncordonNode
	}
	addGateFlags(cmd, gateAction, &gate)
	rootCmd.AddCommand(cmd)
}
