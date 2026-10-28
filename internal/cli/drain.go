package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/internal/output"
	"github.com/jingle2008/toolkit/pkg/models"
)

func addDrainCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	var (
		gate   cliGate
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
				return runDrainStatus(cmd, cfgFile, deps, nodeName, format, gate.DryRun)
			}
			if cmd.Flags().Changed("output") {
				return fmt.Errorf("-o applies only with --status")
			}
			return withMutationSetup(cmd, cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				d := deps(cfg)
				op := ops.Drain(d, opsEnv(cfg, env), nodeName, func(ctx context.Context) error {
					// The drain can run for tens of minutes and the log goes to a
					// file, so echo its milestones on stderr; stdout keeps only
					// the result line.
					errOut := cmd.ErrOrStderr()
					ctx = k8s.WithDrainProgress(ctx, func(line string) {
						_, _ = fmt.Fprintf(errOut, "%s %s\n", time.Now().Format("15:04:05"), line)
					})
					return d.DrainNode(ctx, cfg.KubeConfig, env.KubeContext(), nodeName)
				})
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), gate, op)
			})
		},
	}
	addGateFlags(cmd, mutation.DrainNode, &gate)
	cmd.Flags().BoolVar(&status, "status", false, "Print the drain recorded on the node and exit (changes nothing)")
	cmd.Flags().StringVarP(&format, "output", "o", "yaml", "with --status: yaml|json")
	rootCmd.AddCommand(cmd)
}

// runDrainStatus is `drain <node> --status`: the drain recorded on the
// Node, read-only.
func runDrainStatus(cmd *cobra.Command, cfgFile *string, deps depsFor, nodeName, format string, dryRun bool) error {
	if dryRun {
		return fmt.Errorf("--status changes nothing; --dry-run doesn't apply")
	}
	fmtChoice, err := output.ParseFormatIn(format, output.FormatYAML, output.FormatYAML, output.FormatJSON)
	if err != nil {
		return fmt.Errorf("--status prints yaml or json: %w", err)
	}
	return withCommandSetup(cmd.Context(), cfgFile, "drain-status", standardChecks(true, false, true), func(ctx context.Context, cfg config.Config, env models.Environment) error {
		st, err := deps(cfg).DrainStatus(ctx, cfg.KubeConfig, env.KubeContext(), nodeName)
		if err != nil {
			return fmt.Errorf("get drain status of node/%s: %w", nodeName, err)
		}
		return writeEncoded(cmd.OutOrStdout(), output.Options{Format: fmtChoice, Pretty: true}, st)
	})
}
