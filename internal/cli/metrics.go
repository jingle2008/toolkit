package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// metricsTargetFn is the seam tests use to fake the row and catalog load.
var metricsTargetFn = resolve.MetricsTarget

// addMetricsCommand wires `toolkit metrics <category> <name>`.
func addMetricsCommand(rootCmd *cobra.Command, cfgFile *string) {
	var dac string
	cmd := &cobra.Command{
		Use:   "metrics <category> <name>",
		Short: "Print the OCI Telemetry dashboard URL for a DAC, imported model, GPU workload or inference service",
		Long: `Prints the OCI Telemetry MQL dashboard URL for one row — the dashboard the
TUI's metrics key opens: the last 7 days of the metrics the served model's
capability calls for, scoped to the DAC (dedicated) or the model (on-demand).

<name> matches as in ` + "`toolkit get <category> <name>`" + `: exactly, or
<group>/<name> when rows in different groups share it. An imported model
several DACs serve needs --dac to say which one's dashboard.

Examples:
  toolkit metrics dac <dac-name>
  toolkit metrics inferenceservice <namespace>/<name>
  toolkit metrics importedmodel <name> --dac <dac-namespace>
  open "$(toolkit metrics dac <dac-name>)"`,
		Args: cobra.ExactArgs(2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			names := make([]string, len(telemetry.MetricsCategories))
			for i, c := range telemetry.MetricsCategories {
				names[i] = strings.ToLower(c.String())
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := domain.ParseCategory(args[0])
			if err != nil {
				return fmt.Errorf("unknown category %q", args[0])
			}
			return withCommandSetup(cmd.Context(), cfgFile, "metrics", true, false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
				ld := production.New(ctx, cfg.MetadataFile)
				return emitMetrics(ctx, cmd.OutOrStdout(), ld, cfg, env, cat, args[1], dac, time.Now())
			})
		},
	}
	cmd.Flags().StringVar(&dac, "dac", "", "for an imported model several DACs serve: the DAC namespace whose dashboard to print")
	rootCmd.AddCommand(cmd)
}

// emitMetrics prints the dashboard URL for cat's row named name, as of
// now.
func emitMetrics(
	ctx context.Context, w io.Writer, ld loader.Composite, cfg config.Config, env models.Environment,
	cat domain.Category, name, dac string, now time.Time,
) error {
	item, catalog, warnings, err := metricsTargetFn(ctx, ld, cfg.RepoPath, cfg.KubeConfig, env, cat, name)
	if err != nil {
		return withNameHint(err)
	}
	for _, msg := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	plan, err := telemetry.PlanOn(item, env, catalog, dac)
	if _, ok := errors.AsType[*telemetry.SeveralDACsError](err); ok {
		return fmt.Errorf("%w; pass --dac <namespace>", err)
	}
	if errors.Is(err, telemetry.ErrNoMetrics) {
		return fmt.Errorf("no metrics dashboard for %s %s", cat, name)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, telemetry.DashboardURL(env, plan, now))
	return err
}
