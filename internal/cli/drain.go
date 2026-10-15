package cli

import (
	"context"
	"fmt"
	"time"

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
		Short: "Safely drain a node (cordon, restart single-replica endpoints, evict)",
		Long: `Drain follows the production GPU drain runbook, the same one the
TUI uses:

  1. Cordon <node>.
  2. Refuse if it runs fine-tuning, training, or replicationJob pods.
  3. Restart every InferenceService on it with minReplicas <= 1, so
     the rollout surges a replacement onto another node before the old
     pod stops, and wait (up to 20m) for those pods to leave the node.
  4. Evict the rest (IgnoreAllDaemonSets, DeleteEmptyDirData, pod
     grace periods; up to 10m).

If it fails before eviction starts, the node is uncordoned again
(unless it was already cordoned before the drain); once eviction has
started, the node stays cordoned. A reschedule timeout usually means no
other node has spare GPU capacity.

The command blocks until the drain finishes, printing each milestone
(restarts, what it is waiting on, evictions) on stderr, but the drain itself is
recorded on the Node (annotation toolkit/drain): if the command is
interrupted, running it again resumes where it stopped without repeating
finished steps, and the TUI and MCP server see the same drain. Use this
before terminating a node so workloads relocate cleanly.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]
			return withMutationSetup(cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "drain",
					Kind:    "node",
					Target:  nodeName,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					// The drain can run for tens of minutes and the log goes to a
					// file, so echo its milestones on stderr; stdout keeps only
					// the result line.
					errOut := cmd.ErrOrStderr()
					ctx = k8s.WithDrainProgress(ctx, func(line string) {
						_, _ = fmt.Fprintf(errOut, "%s %s\n", time.Now().Format("15:04:05"), line)
					})
					return drainNodeFn(ctx, cfg.KubeConfig, env.KubeContext(), nodeName)
				})
			})
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Print what would happen and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	rootCmd.AddCommand(cmd)
}
