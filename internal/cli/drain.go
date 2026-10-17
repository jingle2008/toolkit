package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/output"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the k8s calls.
var (
	drainNodeFn   = k8s.DrainNode
	drainStatusFn = k8s.GetDrainStatus
)

func addDrainCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		dryRun bool
		yes    bool
		status bool
		format string
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
before terminating a node so workloads relocate cleanly.

With --status it changes nothing: it prints the drain recorded on the
Node (phase, deadline, what it still waits on, any failure), as MCP's
get_drain_status does. A phase still in progress past its deadline means
no process is driving it: run the drain again to resume.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]
			if status {
				return runDrainStatus(cmd, cfgFile, nodeName, format, dryRun)
			}
			if cmd.Flags().Changed("output") {
				return fmt.Errorf("-o applies only with --status")
			}
			return withMutationSetup(cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action: mutation.DrainNode,
					Target: nodeName,
					DryRun: dryRun,
					Yes:    yes,
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
	cmd.Flags().BoolVar(&status, "status", false, "Print the drain recorded on the node and exit (changes nothing)")
	cmd.Flags().StringVarP(&format, "output", "o", "yaml", "with --status: yaml|json")
	rootCmd.AddCommand(cmd)
}

// runDrainStatus is `drain <node> --status`: the drain recorded on the
// Node, read-only.
func runDrainStatus(cmd *cobra.Command, cfgFile *string, nodeName, format string, dryRun bool) error {
	if dryRun {
		return fmt.Errorf("--status changes nothing; --dry-run doesn't apply")
	}
	fmtChoice, err := output.ParseFormat(format)
	if err != nil {
		return err
	}
	if fmtChoice != output.FormatYAML && fmtChoice != output.FormatJSON {
		return fmt.Errorf("--status prints yaml or json, not %s", fmtChoice)
	}
	return withCommandSetup(cfgFile, "drain-status", true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
		st, err := drainStatusFn(ctx, cfg.KubeConfig, env.KubeContext(), nodeName)
		if err != nil {
			return fmt.Errorf("get drain status of node/%s: %w", nodeName, err)
		}
		return writeEncoded(cmd.OutOrStdout(), output.Options{Format: fmtChoice, Pretty: true}, st)
	})
}
