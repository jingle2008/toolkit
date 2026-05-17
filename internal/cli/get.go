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
		Short: "Print a category's data to stdout (table/json/jsonl/yaml)",
		Long: `Headless equivalent of the TUI's category view.

Examples:
  toolkit get tenant -o json
  toolkit get gpunode -f us-ashburn-1 -o jsonl
  toolkit get dac -o table
  toolkit get basemodel -f cohere -o yaml

Category aliases match the TUI (e.g. "tenant"/"t", "gpunode"/"gn",
"dac", "basemodel"/"bm"). Run with shell completion enabled to
discover them.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return domain.Aliases, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runGet(cfgFile, &format, &noHeaders, &pretty),
	}
	getCmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|jsonl|yaml")
	getCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "omit header row (table only)")
	getCmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON/YAML output")
	_ = getCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "jsonl", "yaml"}, cobra.ShellCompDirectiveNoFileComp
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
		logFormat, logLevel, err := logOptionsFromViper()
		if err != nil {
			return err
		}
		logger, err := logging.NewFileLoggerWithLevel(cfg.Debug, cfg.LogFile, logFormat, logLevel)
		if err != nil {
			return fmt.Errorf("initialize logger: %w", err)
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

// validateGetConfig checks the minimum fields needed to load the
// requested category. Unlike config.Validate (used by the TUI), it
// does not require Category — the positional arg supplies it — and
// only requires KubeConfig for cluster-derived categories.
func validateGetConfig(cfg config.Config, cat domain.Category) error {
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
	if categoryNeedsKubeConfig(cat) && cfg.KubeConfig == "" {
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
	if categoryNeedsKubeConfig(cat) {
		if _, err := os.Stat(cfg.KubeConfig); err != nil {
			return fmt.Errorf("kubeconfig %q not readable: %w", cfg.KubeConfig, err)
		}
	}
	return nil
}

// categoryNeedsKubeConfig reports whether loading cat requires a
// kubeconfig. The TUI loads these lazily from a live cluster;
// everything else comes from the on-disk repo.
func categoryNeedsKubeConfig(cat domain.Category) bool {
	switch cat { //nolint:exhaustive
	case domain.BaseModel, domain.GpuNode, domain.DedicatedAICluster:
		return true
	}
	return false
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
			var partial *terraform.PartialLoadError
			if errors.As(err, &partial) {
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
		return writeMap(w, filterMap(grouped, filter), opts, gpuNodeTable, "pool")
	case domain.DedicatedAICluster:
		grouped, err := ld.LoadDedicatedAIClusters(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load dedicated AI clusters: %w", err)
		}
		return writeMap(w, filterMap(grouped, filter), opts, dacTable, "tenant")
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
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

func filterMap[T models.NamedFilterable](grouped map[string][]T, filter string) map[string][]T {
	if filter == "" {
		return grouped
	}
	return collections.FilterMap(grouped, nil, nil, filter, nil)
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
		return writeMap(w, filterMap(group.LimitTenancyOverrideMap, filter), opts,
			tenancyOverrideTable[models.LimitTenancyOverride], "tenant")
	case domain.ConsolePropertyTenancyOverride:
		return writeMap(w, filterMap(group.ConsolePropertyTenancyOverrideMap, filter), opts,
			tenancyOverrideTable[models.ConsolePropertyTenancyOverride], "tenant")
	case domain.PropertyTenancyOverride:
		return writeMap(w, filterMap(group.PropertyTenancyOverrideMap, filter), opts,
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
		return writeMap(w, filterMap(dataset.ModelArtifactMap, filter), opts, modelArtifactTable, "model")
	case domain.Alias:
		return writeAliases(w, filter, opts)
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

func tenantTable(items []models.Tenant) ([]string, [][]string) {
	headers := []string{"NAME", "IDS", "INTERNAL", "NOTE"}
	rows := make([][]string, 0, len(items))
	for _, t := range items {
		rows = append(rows, []string{
			t.Name,
			strings.Join(t.IDs, ","),
			boolStr(t.IsInternal),
			t.Note,
		})
	}
	return headers, rows
}

func baseModelTable(items []models.BaseModel) ([]string, [][]string) {
	headers := []string{"NAME", "INTERNAL", "VENDOR", "TYPE", "VERSION", "STATUS", "FLAGS"}
	rows := make([][]string, 0, len(items))
	for _, m := range items {
		rows = append(rows, []string{
			m.Name,
			m.InternalName,
			m.Vendor,
			m.Type,
			m.Version,
			m.Status,
			m.GetFlags(),
		})
	}
	return headers, rows
}

func gpuPoolTable(items []models.GpuPool) ([]string, [][]string) {
	headers := []string{"NAME", "SHAPE", "SIZE", "CAPACITY TYPE"}
	rows := make([][]string, 0, len(items))
	for _, p := range items {
		rows = append(rows, []string{
			p.Name,
			p.Shape,
			fmt.Sprintf("%d", p.Size),
			p.CapacityType,
		})
	}
	return headers, rows
}

func gpuNodeTable(grouped map[string][]models.GpuNode) ([]string, [][]string) {
	headers := []string{"POOL", "NAME", "STATUS", "INSTANCE TYPE", "AGE"}
	rows := make([][]string, 0)
	for _, k := range sortedKeys(grouped) {
		for _, n := range grouped[k] {
			rows = append(rows, []string{k, n.Name, n.GetStatus(), n.InstanceType, n.Age})
		}
	}
	return headers, rows
}

func dacTable(grouped map[string][]models.DedicatedAICluster) ([]string, [][]string) {
	headers := []string{"TENANT", "NAME", "STATUS", "TYPE", "UNIT SHAPE", "SIZE", "MODEL"}
	rows := make([][]string, 0)
	for _, k := range sortedKeys(grouped) {
		for _, d := range grouped[k] {
			rows = append(rows, []string{
				k, d.Name, d.Status, d.Type, d.UnitShape, fmt.Sprintf("%d", d.Size), d.ModelName,
			})
		}
	}
	return headers, rows
}

func tenancyOverrideTable[T models.NamedItem](grouped map[string][]T) ([]string, [][]string) {
	headers := []string{"TENANT", "NAME"}
	rows := make([][]string, 0)
	for _, k := range sortedKeys(grouped) {
		for _, v := range grouped[k] {
			rows = append(rows, []string{k, v.GetName()})
		}
	}
	return headers, rows
}

func limitDefinitionTable(items []models.LimitDefinition) ([]string, [][]string) {
	headers := []string{"NAME", "DESCRIPTION", "SCOPE", "DEFAULT MIN", "DEFAULT MAX"}
	rows := make([][]string, 0, len(items))
	for _, d := range items {
		rows = append(rows, []string{d.Name, d.Description, d.Scope, d.DefaultMin, d.DefaultMax})
	}
	return headers, rows
}

func definitionTable[T models.Definition](items []T) ([]string, [][]string) {
	headers := []string{"NAME", "DESCRIPTION"}
	rows := make([][]string, 0, len(items))
	for _, d := range items {
		rows = append(rows, []string{d.GetName(), d.GetDescription()})
	}
	return headers, rows
}

func definitionOverrideTable[T models.DefinitionOverride](items []T) ([]string, [][]string) {
	headers := []string{"NAME", "REGIONS"}
	rows := make([][]string, 0, len(items))
	for _, d := range items {
		rows = append(rows, []string{d.GetName(), strings.Join(d.GetRegions(), ",")})
	}
	return headers, rows
}

func environmentTable(items []models.Environment) ([]string, [][]string) {
	headers := []string{"NAME", "TYPE", "REGION", "REALM"}
	rows := make([][]string, 0, len(items))
	for _, e := range items {
		rows = append(rows, []string{e.GetName(), e.Type, e.Region, e.Realm})
	}
	return headers, rows
}

func serviceTenancyTable(items []models.ServiceTenancy) ([]string, [][]string) {
	headers := []string{"NAME", "REALM", "ENVIRONMENT", "HOME REGION", "REGIONS"}
	rows := make([][]string, 0, len(items))
	for _, s := range items {
		rows = append(rows, []string{s.Name, s.Realm, s.Environment, s.HomeRegion, strings.Join(s.Regions, ",")})
	}
	return headers, rows
}

func limitRegionalOverrideTable(items []models.LimitRegionalOverride) ([]string, [][]string) {
	headers := []string{"NAME", "REGIONS"}
	rows := make([][]string, 0, len(items))
	for _, o := range items {
		rows = append(rows, []string{o.Name, strings.Join(o.Regions, ",")})
	}
	return headers, rows
}

func modelArtifactTable(grouped map[string][]models.ModelArtifact) ([]string, [][]string) {
	headers := []string{"MODEL", "NAME", "GPU CONFIG", "TENSORRT"}
	rows := make([][]string, 0)
	for _, k := range sortedKeys(grouped) {
		for _, a := range grouped[k] {
			rows = append(rows, []string{k, a.Name, a.GetGpuConfig(), a.TensorRTVersion})
		}
	}
	return headers, rows
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
