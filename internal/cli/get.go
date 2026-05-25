package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// addGetCommand wires the `toolkit get <category>` subcommand.
func addGetCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		format     string
		noHeaders  bool
		pretty     bool
		limit      int
		columnsArg string
	)
	getCmd := &cobra.Command{
		Use:   "get <category>",
		Short: "Print a category's data to stdout (table/json/jsonl/yaml/csv/tsv)",
		Long: `Headless equivalent of the TUI's category view.

Examples:
  toolkit get tenant -o json
  toolkit get gpunode -f us-ashburn-1 -o jsonl
  toolkit get dac -o table
  toolkit get basemodel -f cohere -o yaml
  toolkit get tenant -o csv > tenants.csv
  toolkit get gpupool -o tsv | cut -f1,3
  toolkit get tenant --limit 10
  toolkit get gpunode --columns name,status,total,free
  toolkit get basemodel --columns help

Category aliases match the TUI (e.g. "tenant"/"t", "gpunode"/"gn",
"dac", "basemodel"/"bm"). Run with shell completion enabled to
discover them.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return domain.Aliases, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runGet(cfgFile, &format, &noHeaders, &pretty, &limit, &columnsArg),
	}
	getCmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|jsonl|yaml|csv|tsv")
	getCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "omit header row (table/csv/tsv only)")
	getCmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON/YAML output")
	getCmd.Flags().IntVar(&limit, "limit", 0, "max items to render (client-side, applied after the fuzzy --filter match); 0 = unlimited. For grouped categories the cap is across the whole flattened result, not per group.")
	getCmd.Flags().StringVar(&columnsArg, "columns", "",
		"comma-separated column keys (table/csv/tsv only; default: category's Default columns). Use --columns help to list valid keys.")
	_ = getCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "jsonl", "yaml", "csv", "tsv"}, cobra.ShellCompDirectiveNoFileComp
	})
	// Known limitation: this completion func returns the full key list without
	// filtering by the in-progress token (toComplete) or handling the
	// comma-separated value case. In zsh, `--columns name,<TAB>` will offer
	// `name` again (replacing the whole flag value with `name`) rather than
	// completing the suffix after the comma. Proper fix requires splitting on
	// the last comma and filtering by the trailing prefix — left as a future
	// follow-up; bash users get reasonable behavior via shell-side filtering.
	_ = getCmd.RegisterFlagCompletionFunc("columns", func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) < 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return columns.KeysFor(cat), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(getCmd)
}

// runGet is the get-command handler. Its branches are orchestration
// (parse category → parse format → parse columns → read config →
// init logger → dispatch), each producing distinct CLI error
// messages; splitting them into helpers would just shuffle state.
//
//nolint:cyclop // sequential CLI orchestration with one branch per failure mode
func runGet(cfgFile *string, format *string, noHeaders, pretty *bool, limit *int, columnsArg *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return fmt.Errorf("unknown category %q (run `toolkit get -h` for examples)", args[0])
		}

		// --columns help short-circuit: print key/title/default table, exit 0.
		if *columnsArg == "help" {
			headers, rows := columns.HelpTable(cat)
			return output.WriteTable(cmd.OutOrStdout(), headers, rows, output.Options{})
		}

		fmtChoice, err := output.ParseFormat(*format)
		if err != nil {
			return err
		}

		selected, err := parseColumnsFlag(*columnsArg)
		if err != nil {
			return err
		}
		if len(selected) > 0 && !isTableLike(fmtChoice) {
			return fmt.Errorf("--columns has no effect with -o %s; remove the flag or switch to -o table/csv/tsv", fmtChoice)
		}

		// Read the YAML config file (if present) so values like repo_path
		// flow into viper before Unmarshal. Matches what runRootE does
		// for the TUI command — without this, `toolkit get` ignored
		// ~/.config/toolkit/config.yaml.
		if err := readConfigFile(cfgFile); err != nil {
			return err
		}

		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
		if err := validateGetConfig(cfg, cat); err != nil {
			return err
		}

		// Honor the same log_format / log_level config keys the TUI uses so
		// users who configured `log_format: json` for scripting actually get
		// JSON. Logs still go to cfg.LogFile by default, keeping stdout
		// clean for piping.
		logger, err := initLogger(cfg)
		if err != nil {
			return err
		}
		defer func() { _ = logger.Sync() }()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx = logging.WithContext(ctx, logger)

		env := models.Environment{Type: cfg.EnvType, Region: cfg.EnvRegion, Realm: cfg.EnvRealm}
		ld := production.NewLoader(ctx, cfg.MetadataFile)

		filter := strings.ToLower(strings.TrimSpace(cfg.Filter))
		opts := output.Options{Format: fmtChoice, NoHeaders: *noHeaders, Pretty: *pretty}

		return emitCategory(ctx, cmd.OutOrStdout(), ld, cat, cfg, env, filter, *limit, opts, selected)
	}
}

// parseColumnsFlag splits "name, status" → ["name","status"], trimming
// whitespace. Empty tokens (e.g. "name,,status") are an error. An empty
// input returns nil, meaning "use Default columns".
func parseColumnsFlag(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			return nil, fmt.Errorf("--columns: empty token in %q", s)
		}
		out = append(out, t)
	}
	return out, nil
}

func isTableLike(f output.Format) bool {
	switch f {
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		return true
	}
	return false
}

// validateLoaderConfig returns the names of missing required flags
// for any subcommand that needs to call the loader composite. Empty
// slice means the config has enough to reach the loader; subcommands
// may add their own category-specific checks on top (see
// validateGetConfig).
func validateLoaderConfig(cfg config.Config) []string {
	var missing []string
	if cfg.RepoPath == "" {
		missing = append(missing, "--repo_path")
	}
	if cfg.EnvType == "" {
		missing = append(missing, "--env_type")
	}
	if cfg.EnvRegion == "" {
		missing = append(missing, "--env_region")
	}
	if cfg.EnvRealm == "" {
		missing = append(missing, "--env_realm")
	}
	return missing
}

// validateGetConfig checks the minimum fields needed to load the
// requested category. Unlike config.Validate (used by the TUI), it
// does not require Category — the positional arg supplies it — and
// only requires KubeConfig for cluster-derived categories.
func validateGetConfig(cfg config.Config, cat domain.Category) error {
	// Alias is a static dump of domain.Aliases — it never reaches the
	// loader, so don't gate it on repo_path / env_*.
	if cat == domain.Alias {
		return nil
	}
	missing := validateLoaderConfig(cfg)
	if cat.NeedsKubeConfig() && cfg.KubeConfig == "" {
		missing = append(missing, "--kubeconfig")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required setting(s) for `toolkit get %s`: %s\n"+
				"  set them via flags, environment (TOOLKIT_*), or `toolkit init` to scaffold ~/.config/toolkit/config.yaml",
			cat, strings.Join(missing, ", "),
		)
	}
	// For cluster-derived categories the path is guaranteed non-empty
	// (a default of ~/.kube/config is bound by the persistent flag),
	// so stat the file here to fail fast with a clear message instead
	// of letting client-go produce a deep, generic error.
	if cat.NeedsKubeConfig() {
		if _, err := os.Stat(cfg.KubeConfig); err != nil {
			return fmt.Errorf("kubeconfig %q not readable: %w", cfg.KubeConfig, err)
		}
	}
	return nil
}

//nolint:cyclop // a simple per-category switch is clearer than a registry here
func emitCategory(
	ctx context.Context,
	w writer,
	ld loader.Loader,
	cat domain.Category,
	cfg config.Config,
	env models.Environment,
	filter string,
	limit int,
	opts output.Options,
	selected []string,
) error {
	switch cat {
	case domain.Alias:
		// Static enum dump — no loader call. Handled before the default
		// branch so it can run without repo_path or env_* set.
		return writeAliases(w, filter, limit, opts, selected)
	case domain.BaseModel:
		items, err := ld.LoadBaseModels(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load base models: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.BaseModel, env, selected)
	case domain.ImportedModel:
		grouped, err := ld.LoadImportedModels(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load imported models: %w", err)
		}
		// No top-level `tenant` injection: each item carries its
		// own `tenantId` field already (same shape as DAC after
		// the recent wrapper-drop refactor).
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), limit, opts, domain.ImportedModel, env, selected)
	case domain.GpuPool:
		items, err := ld.LoadGpuPools(ctx, cfg.RepoPath, env)
		if err != nil {
			// Partial success: items has the rows we could load; surface
			// the per-source failures on stderr so scripts and LLM
			// consumers know the result is incomplete, then proceed.
			if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
				fmt.Fprintf(os.Stderr, "warning: load gpu pools: %s\n", partial.Error())
			} else {
				return fmt.Errorf("load gpu pools: %w", err)
			}
		}
		// Enrich ActualSize / Status from OCI's ListInstancePools (same
		// step the TUI runs after load). Degrades to placeholder on
		// failure so an offline / no-OCI-auth session still prints the
		// Terraform-derived columns.
		if msg := resolve.EnrichGpuPools(ctx, items, cfg.KubeConfig, env); msg != "" {
			fmt.Fprintf(os.Stderr, "warning: gpu pool enrichment incomplete: %s\n", msg)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.GpuPool, env, selected)
	case domain.GpuNode:
		grouped, err := ld.LoadGpuNodes(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load gpu nodes: %w", err)
		}
		// No top-level `pool` injection: GpuNode.NodePool (json
		// `poolName`) already carries the group key; the loader sets
		// it from the same value used as the map key.
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), limit, opts, domain.GpuNode, env, selected)
	case domain.DedicatedAICluster:
		grouped, err := ld.LoadDedicatedAIClusters(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load dedicated AI clusters: %w", err)
		}
		// No top-level `tenant` injection: the loader keys this map
		// by dac.TenantID (internal/infra/k8s/dac.go:157), which is
		// already the flat `tenantId` field on each value.
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), limit, opts, domain.DedicatedAICluster, env, selected)
	case domain.Tenant,
		domain.LimitTenancyOverride,
		domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride:
		group, err := ld.LoadTenancyOverrideGroup(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load tenancy override group: %w", err)
		}
		return emitTenancyGroup(w, cat, group, filter, limit, opts, env, selected)
	case domain.LimitRegionalOverride:
		items, err := ld.LoadLimitRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load limit regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.LimitRegionalOverride, env, selected)
	case domain.ConsolePropertyRegionalOverride:
		items, err := ld.LoadConsolePropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load console property regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.ConsolePropertyRegionalOverride, env, selected)
	case domain.PropertyRegionalOverride:
		items, err := ld.LoadPropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load property regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.PropertyRegionalOverride, env, selected)
	default:
		dataset, err := ld.LoadDataset(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load dataset: %w", err)
		}
		return emitFromDataset(w, cat, dataset, filter, limit, opts, env, selected)
	}
}

// writer narrows io.Writer to what we actually need so the signature
// stays honest about not closing stdout.
type writer interface {
	Write(p []byte) (int, error)
}

// writeSlice renders a flat slice through the canonical column registry.
// JSON/JSONL/YAML bypass the registry and encode the typed items directly.
// For table/csv/tsv, columns.RenderTable is called with cat and selected
// (the parsed --columns list; empty means "use Default==true columns").
// CSV/TSV additionally route through columns.RenderTableForExport with
// the caller's env so columns marked with ExportRender (DAC and
// ImportedModel Name/Tenant today) emit fully-qualified OCIDs instead
// of raw suffixes. Table output keeps the display-mode Render to
// preserve column-width headroom.
func writeSlice[T any](w writer, items []T, limit int, opts output.Options, cat domain.Category, env models.Environment, selected []string) error {
	items = collections.TruncateSlice(items, limit)
	switch opts.Format {
	case output.FormatJSON:
		return output.WriteJSON(w, items, opts)
	case output.FormatJSONL:
		return output.WriteJSONL(w, items, opts)
	case output.FormatYAML:
		return output.WriteYAML(w, items, opts)
	case output.FormatTable:
		headers, rows, err := columns.RenderTable(cat, items, selected)
		if err != nil {
			return err
		}
		return writeTableLike(w, headers, rows, opts)
	case output.FormatCSV, output.FormatTSV:
		headers, rows, err := columns.RenderTableForExport(cat, items, env.Realm, env.Region, selected)
		if err != nil {
			return err
		}
		return writeTableLike(w, headers, rows, opts)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

// writeTableLike dispatches the table/csv/tsv branches given pre-built
// headers and rows. Shared between writeSlice and the map helpers.
func writeTableLike(w writer, headers []string, rows [][]string, opts output.Options) error {
	switch opts.Format {
	case output.FormatTable:
		return output.WriteTable(w, headers, rows, opts)
	case output.FormatCSV:
		return output.WriteDelimited(w, headers, rows, opts, ',')
	case output.FormatTSV:
		return output.WriteDelimited(w, headers, rows, opts, '\t')
	}
	return fmt.Errorf("writeTableLike: unsupported %q", opts.Format)
}

// writeMap renders a grouped map whose values already carry the group key
// as a struct field (GpuNode.NodePool, DedicatedAICluster.TenantID,
// ImportedModel.TenantID, ModelArtifact.ModelName). JSON/JSONL/YAML use
// output.Flatten so the emitted objects look flat. Table uses
// columns.RenderTable; CSV/TSV route through RenderTableForExport
// with env so OCID-shaped columns emit fully-qualified IDs.
func writeMap[T any](w writer, grouped map[string][]T, limit int, opts output.Options, cat domain.Category, env models.Environment, selected []string) error {
	switch opts.Format {
	case output.FormatJSON, output.FormatJSONL, output.FormatYAML:
		return writeEncoded(w, opts, collections.TruncateSlice(output.Flatten(grouped), limit))
	case output.FormatTable:
		headers, rows, err := columns.RenderTable(cat, grouped, selected)
		if err != nil {
			return err
		}
		rows = collections.TruncateSlice(rows, limit)
		return writeTableLike(w, headers, rows, opts)
	case output.FormatCSV, output.FormatTSV:
		headers, rows, err := columns.RenderTableForExport(cat, grouped, env.Realm, env.Region, selected)
		if err != nil {
			return err
		}
		rows = collections.TruncateSlice(rows, limit)
		return writeTableLike(w, headers, rows, opts)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

// writeEncoded dispatches the json/jsonl/yaml branches for an
// already-flattened items slice.
func writeEncoded(w writer, opts output.Options, items any) error {
	switch opts.Format {
	case output.FormatJSON:
		return output.WriteJSON(w, items, opts)
	case output.FormatJSONL:
		return output.WriteJSONL(w, items, opts)
	case output.FormatYAML:
		return output.WriteYAML(w, items, opts)
	default:
		return fmt.Errorf("unsupported encoded format %q", opts.Format)
	}
}

func emitTenancyGroup(
	w writer,
	cat domain.Category,
	group models.TenancyOverrideGroup,
	filter string,
	limit int,
	opts output.Options,
	env models.Environment,
	selected []string,
) error {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return writeSlice(w, collections.FilterSlice(group.Tenants, nil, filter, nil), limit, opts, domain.Tenant, env, selected)
	case domain.LimitTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.LimitTenancyOverrideMap, filter), limit, opts,
			domain.LimitTenancyOverride, env, selected)
	case domain.ConsolePropertyTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.ConsolePropertyTenancyOverrideMap, filter), limit, opts,
			domain.ConsolePropertyTenancyOverride, env, selected)
	case domain.PropertyTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.PropertyTenancyOverrideMap, filter), limit, opts,
			domain.PropertyTenancyOverride, env, selected)
	default:
		return fmt.Errorf("category %s not in tenancy group", cat)
	}
}

func emitFromDataset(
	w writer,
	cat domain.Category,
	dataset *models.Dataset,
	filter string,
	limit int,
	opts output.Options,
	env models.Environment,
	selected []string,
) error {
	switch cat { //nolint:exhaustive
	case domain.LimitDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.LimitDefinitionGroup.Values, nil, filter, nil),
			limit, opts, domain.LimitDefinition, env, selected)
	case domain.ConsolePropertyDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.ConsolePropertyDefinitionGroup.Values, nil, filter, nil),
			limit, opts, domain.ConsolePropertyDefinition, env, selected)
	case domain.PropertyDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.PropertyDefinitionGroup.Values, nil, filter, nil),
			limit, opts, domain.PropertyDefinition, env, selected)
	case domain.Environment:
		return writeSlice(w,
			collections.FilterSlice(dataset.Environments, nil, filter, nil),
			limit, opts, domain.Environment, env, selected)
	case domain.ServiceTenancy:
		return writeSlice(w,
			collections.FilterSlice(dataset.ServiceTenancies, nil, filter, nil),
			limit, opts, domain.ServiceTenancy, env, selected)
	case domain.ModelArtifact:
		// No top-level `model` injection: ModelArtifact.ModelName
		// (json `model_name`) already carries the group key; the
		// loader sets it from the same Terraform key used for the map.
		return writeMap(w, collections.FilterMapOrAll(dataset.ModelArtifactMap, filter), limit, opts, domain.ModelArtifact, env, selected)
	default:
		return fmt.Errorf("category %s is not supported by `toolkit get`", cat)
	}
}

// aliasView is the JSON/YAML shape for `toolkit get alias`. It matches
// the canonical column set (Name + Aliases joined): one entry per
// category, with the list of aliases for that category. Useful for
// scripts and LLM agents.
type aliasView struct {
	Name    string   `json:"name" yaml:"name"`
	Aliases []string `json:"aliases" yaml:"aliases"`
}

// writeAliases renders the canonical alias list — one row per category
// (TUI shape, spec Decision #4). This is an intentional change from
// the legacy 1-row-per-alias CLI shape.
//
//nolint:cyclop // filter loop + per-format dispatch are intrinsic to the contract
func writeAliases(w writer, filter string, limit int, opts output.Options, selected []string) error {
	cats := make([]domain.Category, 0, len(domain.Categories))
	for _, c := range domain.Categories {
		if c == domain.CategoryUnknown {
			continue
		}
		if filter != "" {
			catName := strings.ToLower(c.String())
			if !strings.Contains(catName, filter) {
				matched := false
				for _, a := range c.GetAliases() {
					if strings.Contains(strings.ToLower(a), filter) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		cats = append(cats, c)
	}
	// Categories are already in registration (enum) order.
	switch opts.Format {
	case output.FormatJSON, output.FormatJSONL, output.FormatYAML:
		items := make([]aliasView, 0, len(cats))
		for _, c := range cats {
			items = append(items, aliasView{Name: c.String(), Aliases: c.GetAliases()})
		}
		return writeEncoded(w, opts, collections.TruncateSlice(items, limit))
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		// Alias is a static enum dump — no env-dependent columns,
		// so passing an empty environment is correct.
		return writeSlice(w, cats, limit, opts, domain.Alias, models.Environment{}, selected)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}
