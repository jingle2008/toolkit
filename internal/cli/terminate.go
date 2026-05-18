package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// terminateInstanceFn is the seam tests use to fake the OCI call.
var terminateInstanceFn = actions.TerminateInstance

func addTerminateCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		dryRun bool
		yes    bool
		ocid   string
	)
	cmd := &cobra.Command{
		Use:   "terminate <node>",
		Short: "Terminate the OCI instance backing a GPU node (destructive).",
		Long: `Submits a TerminateInstance request to OCI for the instance
backing <node>. PreserveBootVolume=false (same default as the TUI's
delete-node flow) — the boot volume is destroyed along with the
instance. Fire-and-forget: the request returns once OCI accepts it.

Destructive: requires explicit --yes; the interactive prompt is
deliberately disabled to prevent reflex "y" answers.

By default <node> is resolved against the live cluster (kube config +
env triple required). Pass --ocid to skip the cluster lookup when you
already know the instance OCID.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("unmarshal config: %w", err)
			}
			needsKube := ocid == ""
			if err := validateMutationConfig(cfg, needsKube); err != nil {
				return err
			}
			logger, err := initLogger(cfg)
			if err != nil {
				return err
			}
			defer func() { _ = logger.Sync() }()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ctx = logging.WithContext(ctx, logger)

			env := models.Environment{
				Type:   cfg.EnvType,
				Region: cfg.EnvRegion,
				Realm:  cfg.EnvRealm,
			}
			return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
				Action:             "terminate",
				Kind:               "node",
				Target:             name,
				Surface:            "cli",
				DryRun:             dryRun,
				Yes:                yes,
				RequireExplicitYes: true,
			}, func(ctx context.Context) error {
				node, err := resolveGpuNode(ctx, cfg, env, name, ocid)
				if err != nil {
					return err
				}
				return terminateInstanceFn(ctx, node, env, logging.FromContext(ctx))
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Required: this action has no interactive prompt")
	cmd.Flags().StringVar(&ocid, "ocid", "", "Skip k8s lookup and target this instance OCID directly")
	rootCmd.AddCommand(cmd)
}
