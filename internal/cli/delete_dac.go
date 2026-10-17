package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the OCI call and the cluster calls.
var (
	deleteDACFn            = actions.DeleteDedicatedAICluster
	planOrphanISVCDeleteFn = k8s.PlanOrphanInferenceServiceDelete
	deleteOrphanISVCFn     = k8s.DeleteOrphanInferenceService
)

func addDeleteCommand(rootCmd *cobra.Command, cfgFile *string) {
	delCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource (destructive)",
	}

	var (
		dryRun bool
		yes    bool
	)
	dacCmd := &cobra.Command{
		Use:   "dac <name>",
		Short: "Delete a dedicated AI cluster (synchronous; polls the work request)",
		Long: `Deletes the DAC and its endpoints. Synchronous: the call
blocks until the work request reports SUCCEEDED or FAILED (10-min
timeout). <name> is the DAC's identifier — the same string the table
in toolkit get dac shows. Uniqueness comes from realm+region+name in
the OCID, so no --tenant flag is needed.

Destructive: requires explicit --yes; the interactive prompt is
deliberately disabled to prevent reflex "y" answers.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withMutationSetup(cfgFile, false, false, true, func(ctx context.Context, _ config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action: mutation.DeleteDAC,
					Target: name,
					DryRun: dryRun,
					Yes:    yes,
				}, func(ctx context.Context) error {
					dac := &models.DedicatedAICluster{Name: name}
					return deleteDACFn(ctx, dac, env, logging.FromContext(ctx))
				})
			})
		},
	}
	dacCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Print what would happen and exit")
	dacCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Required: this action has no interactive prompt")

	delCmd.AddCommand(dacCmd, newDeleteISVCCommand(cfgFile))
	rootCmd.AddCommand(delCmd)
}

// newDeleteISVCCommand wires `toolkit delete inference-service <name>`.
// The target is resolved and the delete planned before runMutation, so a
// refusal happens before any prompt, and the dry-run line, prompt, and
// audit log name the real action: delete, or force-delete for a deletion
// stuck on its finalizer.
func newDeleteISVCCommand(cfgFile *string) *cobra.Command {
	var (
		dryRun    bool
		yes       bool
		namespace string
	)
	cmd := &cobra.Command{
		Use:     "inference-service <name>",
		Aliases: []string{"isvc", "inferenceservice"},
		Short:   "Delete an orphaned InferenceService (its DAC no longer exists)",
		Long: `Removes a dedicated InferenceService whose dedicated AI cluster is
gone — the rows toolkit doctor --relations reports as dangling. Anything
else is refused: a shared endpoint, or one whose DAC still exists in any
DAC API (delete the DAC instead; the control plane owns its endpoints).

An endpoint not yet being deleted is deleted. The OME controller can't
finalize an endpoint whose DAC or model is gone, so the deletion can hang
on its finalizer and leave the namespace Terminating. Run the command
again once the deletion has been pending for 10 minutes: it then
force-deletes, removing the inferenceservice.finalizers finalizer.

Without --namespace, <name> is looked up across all namespaces and must
be unique. Destructive: requires explicit --yes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withMutationSetup(cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				ns, err := resolveInferenceServiceFn(ctx, cfg.KubeConfig, env.KubeContext(), namespace, name)
				if err != nil {
					return err
				}
				outcome, err := planOrphanISVCDeleteFn(ctx, cfg.KubeConfig, env.KubeContext(), ns, name)
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				return runMutation(ctx, cmd.InOrStdin(), out, mutationPlan{
					Action: mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized),
					Target: ns + "/" + name,
					DryRun: dryRun,
					Yes:    yes,
				}, func(ctx context.Context) error {
					done, err := deleteOrphanISVCFn(ctx, cfg.KubeConfig, env.KubeContext(), ns, name)
					if err == nil {
						_, _ = fmt.Fprintln(out, done.Summary(ns+"/"+name))
					}
					return err
				})
			})
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Resolve and check the target, print what would happen, and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Required: this action has no interactive prompt")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace of the InferenceService (default: search all namespaces)")
	return cmd
}
