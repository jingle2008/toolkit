package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the cluster lookup, the restart patch, and the
// OCI soft reset.
var (
	resolveInferenceServiceFn = k8s.ResolveInferenceService
	restartInferenceServiceFn = k8s.RestartInferenceService
	softResetInstanceFn       = oci.SoftResetInstance
)

// addRestartCommand wires `toolkit restart node <node>` and
// `toolkit restart inference-service <name>`. For an InferenceService the
// namespace is resolved before runMutation so the prompt, dry-run line,
// and audit log all carry the full <namespace>/<name> target, and a dry
// run proves the target exists.
func addRestartCommand(rootCmd *cobra.Command, cfgFile *string) {
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
			return withMutationSetup(cfgFile, true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				var ns string
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  mutation.RestartInferenceService,
					Target:  isvcTarget(namespace, name),
					DryRun:  dryRun,
					Yes:     yes,
					Resolve: resolveISVC(cfg, env, namespace, name, &ns),
				}, func(ctx context.Context) error {
					return restartInferenceServiceFn(ctx, cfg.KubeConfig, env.KubeContext(), ns, name)
				})
			})
		},
	}
	isvcCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Resolve the target, print what would happen, and exit")
	isvcCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	// No -n shorthand: every mutation command already uses -n for --dry-run.
	isvcCmd.Flags().StringVar(&namespace, "namespace", "", "Namespace of the InferenceService (default: search all namespaces)")

	restartCmd.AddCommand(newRestartNodeCommand(cfgFile), isvcCmd)
	rootCmd.AddCommand(restartCmd)
}

// newRestartNodeCommand wires `toolkit restart node <node>`: an OCI soft
// reset of the instance backing a GPU node.
func newRestartNodeCommand(cfgFile *string) *cobra.Command {
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
env triple required). Pass --ocid to skip the cluster lookup when you
already know the instance OCID.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodeAction(cmd, cfgFile, mutation.RestartNode, args[0], ocid, dryRun, yes, softResetInstanceFn)
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")
	cmd.Flags().StringVar(&ocid, "ocid", "", "Skip k8s lookup and target this instance OCID directly")
	return cmd
}
