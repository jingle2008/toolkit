package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// addGetCommand wires the `toolkit get <category>` subcommand.
func addGetCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		format    string
		noHeaders bool
		pretty    bool
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

Category aliases match the TUI (e.g. "tenant"/"t", "gpunode"/"gn",
"dac", "basemodel"/"bm"). Run with shell completion enabled to
discover them.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return domain.Aliases, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runGet(cfgFile, &format, &noHeaders, &pretty),
	}
	getCmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|jsonl|yaml|csv|tsv")
	getCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "omit header row (table/csv/tsv only)")
	getCmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON/YAML output")
	_ = getCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "jsonl", "yaml", "csv", "tsv"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(getCmd)
}

func runGet(cfgFile *string, format *string, noHeaders, pretty *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return fmt.Errorf("unknown category %q (run `toolkit get -h` for examples)", args[0])
		}
		fmtChoice, err := output.ParseFormat(*format)
		if err != nil {
			return err
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

		return emitCategory(ctx, cmd.OutOrStdout(), ld, cat, cfg, env, filter, opts)
	}
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
	opts output.Options,
) error {
	switch cat {
	case domain.Alias:
		// Static enum dump — no loader call. Handled before the default
		// branch so it can run without repo_path or env_* set.
		return writeAliases(w, filter, opts)
	case domain.BaseModel:
		items, err := ld.LoadBaseModels(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load base models: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), opts, baseModelTable)
	case domain.GpuPool:
		items, err := ld.LoadGpuPools(ctx, cfg.RepoPath, env)
		if err != nil {
			// Partial success: items has the rows we could load; surface
			// the per-source failures on stderr so scripts and LLM
			// consumers know the result is incomplete, then proceed.
			if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
				fmt.Fprintf(os.Stderr, "warning: %s\n", partial.Error())
			} else {
				return fmt.Errorf("load gpu pools: %w", err)
			}
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), opts, gpuPoolTable)
	case domain.GpuNode:
		grouped, err := ld.LoadGpuNodes(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load gpu nodes: %w", err)
		}
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), opts, gpuNodeTable, "pool")
	case domain.DedicatedAICluster:
		grouped, err := ld.LoadDedicatedAIClusters(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load dedicated AI clusters: %w", err)
		}
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), opts, dacTable, "tenant")
	case domain.Tenant,
		domain.LimitTenancyOverride,
		domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride:
		group, err := ld.LoadTenancyOverrideGroup(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load tenancy override group: %w", err)
		}
		return emitTenancyGroup(w, cat, group, filter, opts)
	case domain.LimitRegionalOverride:
		items, err := ld.LoadLimitRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load limit regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), opts, limitRegionalOverrideTable)
	case domain.ConsolePropertyRegionalOverride:
		items, err := ld.LoadConsolePropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load console property regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), opts, definitionOverrideTable[models.ConsolePropertyRegionalOverride])
	case domain.PropertyRegionalOverride:
		items, err := ld.LoadPropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load property regional overrides: %w", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), opts, definitionOverrideTable[models.PropertyRegionalOverride])
	default:
		dataset, err := ld.LoadDataset(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load dataset: %w", err)
		}
		return emitFromDataset(w, cat, dataset, filter, opts)
	}
}

// writer narrows io.Writer to what we actually need so the signature
// stays honest about not closing stdout.
type writer interface {
	Write(p []byte) (int, error)
}

func writeSlice[T any](
	w writer,
	items []T,
	opts output.Options,
	toTable func([]T) (headers []string, rows [][]string),
) error {
	switch opts.Format {
	case output.FormatJSON:
		return output.WriteJSON(w, items, opts)
	case output.FormatJSONL:
		return output.WriteJSONL(w, items, opts)
	case output.FormatYAML:
		return output.WriteYAML(w, items, opts)
	case output.FormatTable:
		headers, rows := toTable(items)
		return output.WriteTable(w, headers, rows, opts)
	case output.FormatCSV:
		headers, rows := toTable(items)
		return output.WriteDelimited(w, headers, rows, opts, ',')
	case output.FormatTSV:
		headers, rows := toTable(items)
		return output.WriteDelimited(w, headers, rows, opts, '\t')
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

// writeMap renders a grouped slice. For json/jsonl/yaml the input is
// flattened to []map[string]any with `groupField` carrying the original
// map key, so consumers see a uniform array of objects (matches the
// shape MCP tools return). The table path keeps the grouped input so
// the per-category renderer can show a group column.
func writeMap[T any](
	w writer,
	grouped map[string][]T,
	opts output.Options,
	toTable func(map[string][]T) (headers []string, rows [][]string),
	groupField string,
) error {
	switch opts.Format {
	case output.FormatJSON:
		return output.WriteJSON(w, output.FlattenWithKey(grouped, groupField), opts)
	case output.FormatJSONL:
		return output.WriteJSONL(w, output.FlattenWithKey(grouped, groupField), opts)
	case output.FormatYAML:
		return output.WriteYAML(w, output.FlattenWithKey(grouped, groupField), opts)
	case output.FormatTable:
		headers, rows := toTable(grouped)
		return output.WriteTable(w, headers, rows, opts)
	case output.FormatCSV:
		headers, rows := toTable(grouped)
		return output.WriteDelimited(w, headers, rows, opts, ',')
	case output.FormatTSV:
		headers, rows := toTable(grouped)
		return output.WriteDelimited(w, headers, rows, opts, '\t')
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

func emitTenancyGroup(
	w writer,
	cat domain.Category,
	group models.TenancyOverrideGroup,
	filter string,
	opts output.Options,
) error {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return writeSlice(w, collections.FilterSlice(group.Tenants, nil, filter, nil), opts, tenantTable)
	case domain.LimitTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.LimitTenancyOverrideMap, filter), opts,
			tenancyOverrideTable[models.LimitTenancyOverride], "tenant")
	case domain.ConsolePropertyTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.ConsolePropertyTenancyOverrideMap, filter), opts,
			tenancyOverrideTable[models.ConsolePropertyTenancyOverride], "tenant")
	case domain.PropertyTenancyOverride:
		return writeMap(w, collections.FilterMapOrAll(group.PropertyTenancyOverrideMap, filter), opts,
			tenancyOverrideTable[models.PropertyTenancyOverride], "tenant")
	default:
		return fmt.Errorf("category %s not in tenancy group", cat)
	}
}

func emitFromDataset(
	w writer,
	cat domain.Category,
	dataset *models.Dataset,
	filter string,
	opts output.Options,
) error {
	switch cat { //nolint:exhaustive
	case domain.LimitDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.LimitDefinitionGroup.Values, nil, filter, nil),
			opts, limitDefinitionTable)
	case domain.ConsolePropertyDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.ConsolePropertyDefinitionGroup.Values, nil, filter, nil),
			opts, definitionTable[models.ConsolePropertyDefinition])
	case domain.PropertyDefinition:
		return writeSlice(w,
			collections.FilterSlice(dataset.PropertyDefinitionGroup.Values, nil, filter, nil),
			opts, definitionTable[models.PropertyDefinition])
	case domain.Environment:
		return writeSlice(w,
			collections.FilterSlice(dataset.Environments, nil, filter, nil),
			opts, environmentTable)
	case domain.ServiceTenancy:
		return writeSlice(w,
			collections.FilterSlice(dataset.ServiceTenancies, nil, filter, nil),
			opts, serviceTenancyTable)
	case domain.ModelArtifact:
		return writeMap(w, collections.FilterMapOrAll(dataset.ModelArtifactMap, filter), opts, modelArtifactTable, "model")
	default:
		return fmt.Errorf("category %s is not supported by `toolkit get`", cat)
	}
}

type aliasItem struct {
	Alias    string `json:"alias" yaml:"alias"`
	Category string `json:"category" yaml:"category"`
}

func writeAliases(w writer, filter string, opts output.Options) error {
	items := make([]aliasItem, 0, len(domain.Aliases))
	for _, a := range domain.Aliases {
		cat, err := domain.ParseCategory(a)
		if err != nil {
			continue
		}
		catName := cat.String()
		if filter != "" &&
			!strings.Contains(strings.ToLower(a), filter) &&
			!strings.Contains(strings.ToLower(catName), filter) {
			continue
		}
		items = append(items, aliasItem{Alias: a, Category: catName})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Alias < items[j].Alias })
	return writeSlice(w, items, opts, func(items []aliasItem) ([]string, [][]string) {
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			rows = append(rows, []string{it.Alias, it.Category})
		}
		return []string{"ALIAS", "CATEGORY"}, rows
	})
}

// --- Per-category table renderers ---------------------------------

// tableFromSlice builds a (headers, rows) result by mapping each item
// through row. Captures the boilerplate every flat *Table function
// repeats: pre-size the rows slice, loop, append.
func tableFromSlice[T any](items []T, headers []string, row func(T) []string) ([]string, [][]string) {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, row(item))
	}
	return headers, rows
}

// tableFromGrouped iterates grouped's sorted keys and emits one row
// per item, with row called as row(parentKey, item). Captures the
// boilerplate every grouped *Table function repeats.
func tableFromGrouped[T any](grouped map[string][]T, headers []string, row func(key string, item T) []string) ([]string, [][]string) {
	rows := make([][]string, 0)
	for _, k := range sortedKeys(grouped) {
		for _, item := range grouped[k] {
			rows = append(rows, row(k, item))
		}
	}
	return headers, rows
}

func tenantTable(items []models.Tenant) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "IDS", "INTERNAL", "NOTE"},
		func(t models.Tenant) []string {
			return []string{t.Name, strings.Join(t.IDs, ","), boolStr(t.IsInternal), t.Note}
		})
}

func baseModelTable(items []models.BaseModel) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "INTERNAL", "VENDOR", "TYPE", "VERSION", "STATUS", "FLAGS"},
		func(m models.BaseModel) []string {
			return []string{m.Name, m.InternalName, m.Vendor, m.Type, m.Version, m.Status, m.GetFlags()}
		})
}

func gpuPoolTable(items []models.GpuPool) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "SHAPE", "SIZE", "CAPACITY TYPE"},
		func(p models.GpuPool) []string {
			return []string{p.Name, p.Shape, fmt.Sprintf("%d", p.Size), p.CapacityType}
		})
}

func gpuNodeTable(grouped map[string][]models.GpuNode) ([]string, [][]string) {
	return tableFromGrouped(grouped,
		[]string{"POOL", "NAME", "STATUS", "INSTANCE TYPE", "AGE"},
		func(k string, n models.GpuNode) []string {
			return []string{k, n.Name, n.GetStatus(), n.InstanceType, n.Age}
		})
}

func dacTable(grouped map[string][]models.DedicatedAICluster) ([]string, [][]string) {
	return tableFromGrouped(grouped,
		[]string{"TENANT", "NAME", "STATUS", "TYPE", "UNIT SHAPE", "SIZE", "MODEL"},
		func(k string, d models.DedicatedAICluster) []string {
			return []string{k, d.Name, d.Status, d.Type, d.UnitShape, fmt.Sprintf("%d", d.Size), d.ModelName}
		})
}

func tenancyOverrideTable[T models.NamedItem](grouped map[string][]T) ([]string, [][]string) {
	return tableFromGrouped(grouped,
		[]string{"TENANT", "NAME"},
		func(k string, v T) []string { return []string{k, v.GetName()} })
}

func limitDefinitionTable(items []models.LimitDefinition) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "DESCRIPTION", "SCOPE", "DEFAULT MIN", "DEFAULT MAX"},
		func(d models.LimitDefinition) []string {
			return []string{d.Name, d.Description, d.Scope, d.DefaultMin, d.DefaultMax}
		})
}

func definitionTable[T models.Definition](items []T) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "DESCRIPTION"},
		func(d T) []string { return []string{d.GetName(), d.GetDescription()} })
}

func definitionOverrideTable[T models.DefinitionOverride](items []T) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "REGIONS"},
		func(d T) []string { return []string{d.GetName(), strings.Join(d.GetRegions(), ",")} })
}

func environmentTable(items []models.Environment) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "TYPE", "REGION", "REALM"},
		func(e models.Environment) []string {
			return []string{e.GetName(), e.Type, e.Region, e.Realm}
		})
}

func serviceTenancyTable(items []models.ServiceTenancy) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "REALM", "ENVIRONMENT", "HOME REGION", "REGIONS"},
		func(s models.ServiceTenancy) []string {
			return []string{s.Name, s.Realm, s.Environment, s.HomeRegion, strings.Join(s.Regions, ",")}
		})
}

func limitRegionalOverrideTable(items []models.LimitRegionalOverride) ([]string, [][]string) {
	return tableFromSlice(items,
		[]string{"NAME", "REGIONS"},
		func(o models.LimitRegionalOverride) []string {
			return []string{o.Name, strings.Join(o.Regions, ",")}
		})
}

func modelArtifactTable(grouped map[string][]models.ModelArtifact) ([]string, [][]string) {
	return tableFromGrouped(grouped,
		[]string{"MODEL", "NAME", "GPU CONFIG", "TENSORRT"},
		func(k string, a models.ModelArtifact) []string {
			return []string{k, a.Name, a.GetGpuConfig(), a.TensorRTVersion}
		})
}

func sortedKeys[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
