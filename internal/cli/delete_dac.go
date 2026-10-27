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

func addDeleteCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	delCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource (destructive)",
	}

	var gate cliGate
	dacCmd := &cobra.Command{
		Use:   "dac <name>",
		Short: "Delete a dedicated AI cluster (synchronous; polls the work request)",
		Long: `Deletes the DAC and its endpoints. It is looked up in OCI first,
so --dry-run proves it exists and reports its state. Synchronous: the call
blocks until the work request reports SUCCEEDED or FAILED (10-min
timeout). <name> is the DAC's identifier — the same string the table
in toolkit get dac shows. Uniqueness comes from realm+region+name in
the OCID, so no --tenant flag is needed.

Destructive: requires explicit --yes; the interactive prompt is
deliberately disabled to prevent reflex "y" answers.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withMutationSetup(cmd.Context(), cfgFile, false, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), gate,
					ops.DeleteDAC(deps(cfg), opsEnv(cfg, env), name))
			})
		},
	}
	addGateFlags(dacCmd, mutation.DeleteDAC, &gate)

	delCmd.AddCommand(dacCmd, newDeleteISVCCommand(cfgFile, deps))
	rootCmd.AddCommand(delCmd)
}

// newDeleteISVCCommand wires `toolkit delete inference-service <name>`.
// The target is resolved and the delete planned before runMutation, so a
// refusal happens before any prompt, and the dry-run line, prompt, and
// audit log name the real action: delete, or force-delete for a deletion
// stuck on its finalizer.
func newDeleteISVCCommand(cfgFile *string, deps depsFor) *cobra.Command {
	var (
		gate      cliGate
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
			return withMutationSetup(cmd.Context(), cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				out := cmd.OutOrStdout()
				// Planning also checks it's an orphan, and picks delete or
				// force-delete.
				op, done := ops.DeleteOrphanISVC(deps(cfg), opsEnv(cfg, env), namespace, name)
				perform := op.Perform
				op.Perform = func(ctx context.Context, p mutation.Plan) error {
					err := perform(ctx, p)
					if err == nil {
						_, _ = fmt.Fprintln(out, done.Summary(p.Target))
					}
					return err
				}
				return runMutation(ctx, cmd.InOrStdin(), out, gate, op)
			})
		},
	}
	addGateFlags(cmd, mutation.DeleteInferenceService, &gate)
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace of the InferenceService (default: search all namespaces)")
	return cmd
}
