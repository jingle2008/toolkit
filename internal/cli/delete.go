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

// deleteDACFn is the seam tests use to fake the OCI call.
var deleteDACFn = actions.DeleteDedicatedAICluster

func addDeleteCommand(rootCmd *cobra.Command, cfgFile *string) {
	delCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource (destructive).",
	}

	var (
		dryRun bool
		yes    bool
	)
	dacCmd := &cobra.Command{
		Use:   "dac <name>",
		Short: "Delete a dedicated AI cluster (synchronous; polls work request).",
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
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("unmarshal config: %w", err)
			}
			if err := validateMutationConfig(cfg, false); err != nil {
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
				Action:             "delete",
				Kind:               "dac",
				Target:             name,
				Surface:            "cli",
				DryRun:             dryRun,
				Yes:                yes,
				RequireExplicitYes: true,
			}, func(ctx context.Context) error {
				dac := &models.DedicatedAICluster{Name: name}
				return deleteDACFn(ctx, dac, env, logging.FromContext(ctx))
			})
		},
	}
	dacCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen and exit")
	dacCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Required: this action has no interactive prompt")

	delCmd.AddCommand(dacCmd)
	rootCmd.AddCommand(delCmd)
}
