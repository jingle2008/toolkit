package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the cluster lookup and the restart patch.
var (
	resolveInferenceServiceFn = k8s.ResolveInferenceService
	restartInferenceServiceFn = k8s.RestartInferenceService
)

// addRestartCommand wires `toolkit restart inference-service <name>`.
// The namespace is resolved before runMutation so the prompt, dry-run
// line, and audit log all carry the full <namespace>/<name> target, and
// a dry run proves the target exists.
func addRestartCommand(rootCmd *cobra.Command, cfgFile *string) {
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Trigger a rolling restart of a resource",
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
				ns, err := resolveInferenceServiceFn(ctx, cfg.KubeConfig, env.KubeContext(), namespace, name)
				if err != nil {
					return err
				}
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "restart",
					Kind:    "inference_service",
					Target:  ns + "/" + name,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
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

	restartCmd.AddCommand(isvcCmd)
	rootCmd.AddCommand(restartCmd)
}
