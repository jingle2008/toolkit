package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// addRestartCommand wires `toolkit restart node <node>` and
// `toolkit restart inference-service <name>`. For an InferenceService the
// namespace is resolved before runMutation so the prompt, dry-run line,
// and audit log all carry the full <namespace>/<name> target, and a dry
// run proves the target exists.
func addRestartCommand(rootCmd *cobra.Command, cfgFile *string, deps depsFor) {
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart a resource: a GPU node's instance, or an InferenceService",
	}

	var (
		dryRun    bool
		yes       bool
		namespace string
	)
	isvcCmd := &cobra.Command{
		Use:     "inference-service <name>",
		Aliases: []string{"isvc", "inferenceservice"},
		Short:   "Rolling-restart an OME InferenceService",
		Long: `Stamps the InferenceService's restartedAt annotation with the
current UTC time; the OME controller rolls the serving pods in response.
Fire-and-forget: the call returns once the API server accepts the patch.

Without --namespace, <name> is looked up across all namespaces and must
be unique; an ambiguous name errors with the candidate namespaces.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return withMutationSetup(cmd.Context(), cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cliGate{DryRun: dryRun, Yes: yes},
					ops.RestartISVC(deps(cfg), opsEnv(cfg, env), namespace, name))
			})
		},
	}
	isvcCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Resolve the target, print what would happen, and exit")
	isvcCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	// No -n shorthand: every mutation command already uses -n for --dry-run.
	isvcCmd.Flags().StringVar(&namespace, "namespace", "", "Namespace of the InferenceService (default: search all namespaces)")

	restartCmd.AddCommand(newRestartNodeCommand(cfgFile, deps), isvcCmd)
	rootCmd.AddCommand(restartCmd)
}

// newRestartNodeCommand wires `toolkit restart node <node>`: an OCI soft
// reset of the instance backing a GPU node.
func newRestartNodeCommand(cfgFile *string, deps depsFor) *cobra.Command {
	var (
		dryRun bool
		yes    bool
		ocid   string
	)
	cmd := &cobra.Command{
		Use:   "node <node>",
		Short: "Soft-reset a GPU node's underlying OCI instance",
		Long: `Submits a soft-reset (graceful restart) request to OCI for the
instance backing <node>. Fire-and-forget: the call returns as soon as
OCI accepts the request; check status via the OCI console or any
wait-flow tooling.

By default <node> is resolved against the live cluster (kube config +
env triple required). Pass --ocid to target an instance directly, without
looking the node up (e.g. a node that left the cluster). The instance must
be a member of one of the env's GPU instance pools (per --repo-path,
checked in OCI, in the GPU compartment the cluster's GPU nodes are in)
or the command refuses; the OCID is then the target the
prompt, dry run and audit log name.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeAction(cmd, cfgFile, deps, args[0], ocid, cliGate{DryRun: dryRun, Yes: yes}, ops.RestartNode)
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	cmd.Flags().StringVar(&ocid, "ocid", "", "Target this instance OCID without looking the node up; it must be in one of the env's GPU pools (needs --repo-path)")
	return cmd
}
