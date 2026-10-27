package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/output"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// addGetCommand wires the `toolkit get <category>` subcommand.
func addGetCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		format          string
		noHeaders       bool
		pretty          bool
		limit           int
		selectedColumns string
		relatedTo       string
		faulty          bool
	)
	getCmd := &cobra.Command{
		Use:   "get <category> [name]",
		Short: "Print a category's data, or one row by name, to stdout (table/json/jsonl/yaml/csv/tsv)",
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
  toolkit get gpuworkload --related-to dac/<dac-name>
  toolkit get inferenceservice --related-to servingruntime/<runtime> -o json
  toolkit get gpunode --faulty
  toolkit get inferenceservice --faulty -o json   # e.g. orphans to delete
  toolkit get dac <dac-name>                       # one row, as YAML
  toolkit get inferenceservice <ns>/<name> -o json # <group>/<name> when a name is shared

With a name, get prints that one row (default -o yaml): the name must
match the row's first column exactly, or be <group>/<name> when rows in
different groups share it (<realm>/<name> for a service tenancy). A
model artifact built for several GPU configs is one row per config, all
with the same name: name one as <model>/<name>@<config> (e.g. @2xH100).
No match, or several, is an error that lists what to pass instead.

Category aliases match the TUI (e.g. "tenant"/"t", "gpunode"/"gn",
"dac", "basemodel"/"bm"). Run with shell completion enabled to
discover them.`,
		Args: cobra.RangeArgs(1, 2),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp // a row name: no completion
			}
			return domain.AllAliases(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runGet(cfgFile, &format, &noHeaders, &pretty, &limit, &selectedColumns, &relatedTo, &faulty),
	}
	getCmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|jsonl|yaml|csv|tsv")
	getCmd.Flags().BoolVar(&noHeaders, "no-headers", false, "omit header row (table/csv/tsv only)")
	getCmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON/YAML output")
	getCmd.Flags().IntVar(&limit, "limit", 0, "max items to render (client-side, applied after the fuzzy --filter match); 0 = unlimited. For grouped categories the cap is across the whole flattened result, not per group.")
	getCmd.Flags().StringVar(&relatedTo, "related-to", "",
		"only rows related to this parent row, as <category>/<name> (e.g. dac/<name>, tenant/<name>); "+
			"the same relations as the TUI's drill-in. The name matches the parent row's first column exactly (case-sensitive). "+
			"Combines with --filter.")
	_ = getCmd.RegisterFlagCompletionFunc("related-to", completeRelatedTo)
	getCmd.Flags().BoolVar(&faulty, "faulty", false,
		"only faulty rows, as the TUI's Ctrl+Z shows them: rows in a bad state (an unready endpoint, a pool short of its size, "+
			"a deleting tenant, ...) or naming a parent row that doesn't exist (what doctor --relations reports). "+
			"Loads every category, so it needs the cluster. Combines with --filter and --related-to.")
	getCmd.Flags().StringVar(&selectedColumns, "columns", "",
		"comma-separated column keys (table/csv/tsv only; default: all columns). Use --columns help to list valid keys.")
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
func runGet(cfgFile *string, format *string, noHeaders, pretty *bool, limit *int, selectedColumns, relatedTo *string, faulty *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cat, err := domain.ParseCategory(args[0])
		if err != nil {
			return fmt.Errorf("unknown category %q (run `toolkit get -h` for examples)", args[0])
		}
		related, err := parseRelatedTo(*relatedTo, cat)
		if err != nil {
			return err
		}
		item, err := itemArg(cmd, args, cat, format)
		if err != nil {
			return err
		}

		// --columns help short-circuit: print key/title/default table, exit 0.
		if *selectedColumns == "help" {
			headers, rows := columns.HelpTable(cat)
			return output.WriteTable(cmd.OutOrStdout(), headers, rows, output.Options{})
		}

		fmtChoice, err := output.ParseFormat(*format)
		if err != nil {
			return err
		}

		selected, err := parseColumnsFlag(*selectedColumns)
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
		rawType, rawRegion := cfg.EnvType, cfg.EnvRegion
		if err := cfg.Normalize(); err != nil {
			return err
		}
		if err := validateGetRequest(cfg, cat, *faulty, *limit); err != nil {
			return err
		}

		// Honor the same log_format / log_level config keys the TUI uses so
		// users who configured `log-format: json` for scripting actually get
		// JSON. Logs still go to cfg.LogFile by default, keeping stdout
		// clean for piping.
		logger, logFile, err := initLogger(cfg)
		if err != nil {
			return err
		}
		logger = logger.With("cmd", "get")
		defer func() { _ = logFile.Close() }()
		logEnvAliases(logger, rawType, rawRegion, cfg)

		ctx := logging.WithContext(cmd.Context(), logger)
		if err := checkGetEnvironment(ctx, cfg, cat); err != nil {
			return err
		}

		env := models.Environment{Type: cfg.EnvType, Region: cfg.EnvRegion, Realm: cfg.EnvRealm}
		ld := production.New(ctx, cfg.MetadataFile)

		filter := strings.ToLower(strings.TrimSpace(cfg.Filter))
		opts := output.Options{Format: fmtChoice, NoHeaders: *noHeaders, Pretty: *pretty, Item: item}

		emit := emitCategory
		if *faulty {
			emit = emitFaulty
		}
		return emit(ctx, cmd.OutOrStdout(), ld, cat, cfg, env, filter, related, *limit, opts, selected)
	}
}

// parseColumnsFlag splits "name, status" → ["name","status"], trimming
// whitespace. Empty tokens (e.g. "name,,status") are an error. An empty
// input returns nil, meaning "render every column".
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
		missing = append(missing, "--repo-path")
	}
	if cfg.EnvType == "" {
		missing = append(missing, "--env-type")
	}
	if cfg.EnvRegion == "" {
		missing = append(missing, "--env-region")
	}
	if cfg.EnvRealm == "" {
		missing = append(missing, "--env-realm")
	}
	return missing
}

// validateGetRequest checks --limit (0 is unlimited; a negative one was
// read as unlimited too, though the help didn't say so), then
// validateGetConfig and validateFaulty.
func validateGetRequest(cfg config.Config, cat domain.Category, faulty bool, limit int) error {
	if limit < 0 {
		return fmt.Errorf("--limit must be 0 (unlimited) or more, got %d", limit)
	}
	if err := validateGetConfig(cfg, cat); err != nil {
		return err
	}
	return validateFaulty(cfg, cat, faulty)
}

// validateGetConfig checks the minimum fields needed to load the
// requested category. Unlike config.Validate (used by the TUI), it
// does not require Category — the positional arg supplies it — and
// only requires KubeConfig for cluster-derived categories.
func validateGetConfig(cfg config.Config, cat domain.Category) error {
	// Alias is a static dump of domain.AllAliases() — it never reaches the
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

// parseRelatedTo parses --related-to's <category>/<name> into the parent
// scope rows of cat must relate to. Empty means no narrowing. The parent
// category must scope cat through a domain relation, and the name is kept
// verbatim: it matches the parent row's first column exactly, as a TUI
// drill-in does (tenant names can differ only by case).
func parseRelatedTo(s string, cat domain.Category) (*domain.Scope, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // no --related-to: no scope, not an error
	}
	catName, name, ok := strings.Cut(s, "/")
	if !ok || name == "" {
		return nil, fmt.Errorf("--related-to: want <category>/<name>, got %q", s)
	}
	parent, err := domain.ParseCategory(catName)
	if err != nil {
		return nil, fmt.Errorf("--related-to: unknown category %q", catName)
	}
	if !parent.IsScopeOf(cat) {
		scopers := cat.ScopedBy()
		if len(scopers) == 0 {
			return nil, fmt.Errorf("--related-to: %s rows are not related to any other category", cat)
		}
		return nil, fmt.Errorf("--related-to: %s is not related to %s; use one of: %s",
			parent, cat, strings.Join(relatedToPrefixes(scopers), ", "))
	}
	return &domain.Scope{Category: parent, Name: name}, nil
}

// relatedToPrefixes renders categories as --related-to prefixes
// ("dedicatedaicluster/").
func relatedToPrefixes(cats []domain.Category) []string {
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = strings.ToLower(c.String()) + "/"
	}
	return out
}

// completeRelatedTo completes --related-to with the <category>/ prefixes
// valid for the category being listed; names are left to the user.
func completeRelatedTo(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cat, err := domain.ParseCategory(args[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return relatedToPrefixes(cat.ScopedBy()), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// relatedOrFiltered applies --related-to (when set) and --filter to a
// grouped category, through the same join keys as a TUI drill-in.
// Without --related-to it is exactly FilterMapOrAll, so output is
// unchanged.
func relatedOrFiltered[T models.NamedFilterable](
	keys relations.Grouped[T], grouped map[string][]T, cat domain.Category, related *domain.Scope, filter string,
) map[string][]T {
	if related == nil {
		return collections.FilterMapOrAll(grouped, filter)
	}
	out := keys.Filter(grouped, related, filter, nil)
	if countGrouped(out) == 0 && (filter == "" || countGrouped(keys.Filter(grouped, related, "", nil)) == 0) {
		warnNoneRelated(cat, related)
	}
	return out
}

// countGrouped is how many rows grouped holds.
func countGrouped[T any](grouped map[string][]T) int {
	n := 0
	for _, items := range grouped {
		n += len(items)
	}
	return n
}

// relatedSlice is relatedOrFiltered for a flat category.
func relatedSlice[T models.NamedFilterable](
	keys relations.Flat[T], items []T, cat domain.Category, related *domain.Scope, filter string,
) []T {
	out := collections.FilterSlice(items, nil, filter, keys.Keep(related))
	if related != nil && len(out) == 0 &&
		(filter == "" || len(collections.FilterSlice(items, nil, "", keys.Keep(related))) == 0) {
		warnNoneRelated(cat, related)
	}
	return out
}

// warnNoneRelated notes on stderr that --related-to matched nothing, so a
// mistyped name isn't mistaken for "nothing related". stdout still gets
// the (empty) result for scripts. Callers skip it when rows are related
// and only --filter hid them: the name is fine then.
func warnNoneRelated(cat domain.Category, related *domain.Scope) {
	fmt.Fprintf(os.Stderr, "warning: no %s rows related to %s/%s (the name must match the %s's first column exactly)\n",
		cat, strings.ToLower(related.Category.String()), related.Name, related.Category)
}

// tenantDataset loads the env's tenants for resolving tenant-owned rows.
// A load failure is a warning, not an error: the rows still print, just
// keyed by raw tenancy-id with no owner (the pre-resolution shape).
func tenantDataset(ctx context.Context, ld loader.Composite, cfg config.Config, env models.Environment) *models.Dataset {
	ds, err := resolve.TenantDataset(ctx, ld, cfg.RepoPath, env)
	if err != nil {
		logging.FromContext(ctx).Warn("tenant resolution unavailable", "error", err)
		fmt.Fprintf(os.Stderr, "warning: tenant names unresolved: %s\n", err)
	}
	return ds
}

// workloadOwnersDataset is tenantDataset with the DACs too: they give a
// GPU pod without a tenancy-id label its tenant. A failed DAC load only
// leaves those pods unowned.
func workloadOwnersDataset(ctx context.Context, ld loader.Composite, cfg config.Config, env models.Environment) *models.Dataset {
	ds := tenantDataset(ctx, ld, cfg, env)
	if dacs, err := ld.LoadDedicatedAIClusters(ctx, cfg.KubeConfig, env); err == nil {
		ds.SetDedicatedAIClusterMap(dacs)
	} else {
		logging.FromContext(ctx).Warn("dacs for workload tenants unavailable", "error", err)
	}
	return ds
}

//nolint:cyclop // a simple per-category switch is clearer than a registry here
func emitCategory(
	ctx context.Context,
	w writer,
	ld loader.Composite,
	cat domain.Category,
	cfg config.Config,
	env models.Environment,
	filter string,
	related *domain.Scope,
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
		return emitFlat(w, items, err, "base models", filter, limit, opts, domain.BaseModel, env, selected)
	case domain.ServingRuntime:
		items, err := ld.LoadServingRuntimes(ctx, cfg.KubeConfig, env)
		return emitFlat(w, items, err, "serving runtimes", filter, limit, opts, domain.ServingRuntime, env, selected)
	case domain.ImportedModel:
		grouped, err := ld.LoadImportedModels(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load imported models: %w", err)
		}
		// Re-key by tenant name and set each item's `owner`, as the
		// TUI does; each item still carries its raw `tenantId`.
		ds := tenantDataset(ctx, ld, cfg, env)
		ds.SetImportedModelMap(grouped)
		return writeMap(w, relatedOrFiltered(relations.ImportedModel, ds.ImportedModelMap, domain.ImportedModel, related, filter),
			limit, opts, domain.ImportedModel, env, selected)
	case domain.GPUPool:
		items, err := ld.LoadGPUPools(ctx, cfg.RepoPath, env)
		if err != nil {
			// Partial success: items has the rows we could load; surface
			// the per-source failures on stderr so scripts and LLM
			// consumers know the result is incomplete, then proceed.
			if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
				logging.FromContext(ctx).Warn("load gpu pools: partial failure", "error", partial)
				fmt.Fprintf(os.Stderr, "warning: load gpu pools: %s\n", partial.Error())
			} else {
				return fmt.Errorf("load gpu pools: %w", err)
			}
		}
		// Enrich ActualSize / Status from OCI's ListInstancePools (same
		// step the TUI runs after load). Degrades to placeholder on
		// failure so an offline / no-OCI-auth session still prints the
		// Terraform-derived columns.
		if err := resolve.EnrichGPUPools(ctx, items, cfg.KubeConfig, env); err != nil {
			logging.FromContext(ctx).Warn("gpu pool enrichment incomplete", "error", err)
			fmt.Fprintf(os.Stderr, "warning: gpu pool enrichment incomplete: %s\n", err)
		}
		return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, domain.GPUPool, env, selected)
	case domain.GPUNode:
		grouped, err := ld.LoadGPUNodesByPool(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load gpu nodes: %w", err)
		}
		// No top-level `pool` injection: GPUNode.NodePool (json
		// `poolName`) already carries the group key; the loader sets
		// it from the same value used as the map key.
		return writeMap(w, relatedOrFiltered(relations.GPUNode, grouped, domain.GPUNode, related, filter), limit, opts, domain.GPUNode, env, selected)
	case domain.GPUWorkload:
		grouped, err := ld.LoadGPUWorkloadsByNode(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load gpu workloads: %w", err)
		}
		ds := workloadOwnersDataset(ctx, ld, cfg, env)
		ds.SetGPUWorkloadMap(grouped)
		return writeMap(w, relatedOrFiltered(relations.GPUWorkload, ds.GPUWorkloadMap, domain.GPUWorkload, related, filter),
			limit, opts, domain.GPUWorkload, env, selected)
	case domain.DedicatedAICluster:
		grouped, err := ld.LoadDedicatedAIClusters(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load dedicated AI clusters: %w", err)
		}
		// Re-key from raw tenancy-id to tenant name and set each
		// item's `owner`, as the TUI does; unresolved tenancies keep
		// the raw key. Each item still carries its raw `tenantId`.
		ds := tenantDataset(ctx, ld, cfg, env)
		ds.SetDedicatedAIClusterMap(grouped)
		return writeMap(w, relatedOrFiltered(relations.DedicatedAICluster, ds.DedicatedAIClusterMap, domain.DedicatedAICluster, related, filter),
			limit, opts, domain.DedicatedAICluster, env, selected)
	case domain.InferenceService:
		grouped, err := ld.LoadInferenceServices(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load inference services: %w", err)
		}
		// No top-level `namespace` injection: InferenceService.Namespace
		// already carries the group key. Owners are resolved in place.
		ds := tenantDataset(ctx, ld, cfg, env)
		ds.SetInferenceServiceMap(grouped)
		return writeMap(w, relatedOrFiltered(relations.InferenceService, ds.InferenceServiceMap, domain.InferenceService, related, filter),
			limit, opts, domain.InferenceService, env, selected)
	case domain.Tenant,
		domain.LimitTenancyOverride,
		domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride:
		group, err := ld.LoadTenancyOverrideGroup(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load tenancy override group: %w", err)
		}
		return emitTenancyGroup(w, cat, group, filter, related, limit, opts, env, selected)
	case domain.LimitRegionalOverride:
		items, err := ld.LoadLimitRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load limit regional overrides: %w", err)
		}
		return writeSlice(w, relatedSlice(relations.LimitRegionalOverride, items, domain.LimitRegionalOverride, related, filter),
			limit, opts, domain.LimitRegionalOverride, env, selected)
	case domain.ConsolePropertyRegionalOverride:
		items, err := ld.LoadConsolePropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load console property regional overrides: %w", err)
		}
		return writeSlice(w, relatedSlice(relations.ConsolePropertyRegionalOverride, items, domain.ConsolePropertyRegionalOverride, related, filter),
			limit, opts, domain.ConsolePropertyRegionalOverride, env, selected)
	case domain.PropertyRegionalOverride:
		items, err := ld.LoadPropertyRegionalOverrides(ctx, cfg.RepoPath, env)
		if err != nil {
			return fmt.Errorf("load property regional overrides: %w", err)
		}
		return writeSlice(w, relatedSlice(relations.PropertyRegionalOverride, items, domain.PropertyRegionalOverride, related, filter),
			limit, opts, domain.PropertyRegionalOverride, env, selected)
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
// (the parsed --columns list; empty means "render every column").
// CSV/TSV additionally route through columns.RenderTableForExport with
// the caller's env so columns marked with RenderForExport (DAC and
// ImportedModel Name/Tenant today) emit fully-qualified OCIDs instead
// of raw suffixes. Table output keeps the display-mode Render to
// preserve column-width headroom.
/*
emitFlat handles the load-error check, filter and write for an
ungrouped category, so each case in emitCategory stays a single
expression. Taking loadErr as a parameter rather than checking at the
call site is what keeps the branch out of emitCategory — that switch is
already at the cognitive-complexity ceiling, and every flat category
added to it would otherwise cost another branch.
*/
func emitFlat[T models.NamedFilterable](
	w writer,
	items []T,
	loadErr error,
	what string,
	filter string,
	limit int,
	opts output.Options,
	cat domain.Category,
	env models.Environment,
	selected []string,
) error {
	if loadErr != nil {
		return fmt.Errorf("load %s: %w", what, loadErr)
	}
	return writeSlice(w, collections.FilterSlice(items, nil, filter, nil), limit, opts, cat, env, selected)
}

func writeSlice[T any](w writer, items []T, limit int, opts output.Options, cat domain.Category, env models.Environment, selected []string) error {
	if opts.Item != "" {
		it, err := pickFlat(cat, items, opts.Item)
		if err != nil {
			return err
		}
		return writeOne(w, it, opts, func(o output.Options) error {
			return writeSlice(w, []T{it}, 0, o, cat, env, selected)
		})
	}
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
// as a struct field (GPUNode.NodePool, DedicatedAICluster.TenantID,
// ImportedModel.TenantID, ModelArtifact.ModelName). JSON/JSONL/YAML use
// output.Flatten so the emitted objects look flat. Table uses
// columns.RenderTable; CSV/TSV route through RenderTableForExport
// with env so OCID-shaped columns emit fully-qualified IDs.
func writeMap[T any](w writer, grouped map[string][]T, limit int, opts output.Options, cat domain.Category, env models.Environment, selected []string) error {
	if opts.Item != "" {
		group, it, err := pickGrouped(cat, grouped, opts.Item)
		if err != nil {
			return err
		}
		return writeOne(w, it, opts, func(o output.Options) error {
			return writeMap(w, map[string][]T{group: {it}}, 0, o, cat, env, selected)
		})
	}
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
	related *domain.Scope,
	limit int,
	opts output.Options,
	env models.Environment,
	selected []string,
) error {
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return writeSlice(w, collections.FilterSlice(group.Tenants, nil, filter, nil), limit, opts, domain.Tenant, env, selected)
	case domain.LimitTenancyOverride:
		return writeMap(w, relatedOrFiltered(relations.LimitTenancyOverride, group.LimitTenancyOverrideMap, domain.LimitTenancyOverride, related, filter),
			limit, opts, domain.LimitTenancyOverride, env, selected)
	case domain.ConsolePropertyTenancyOverride:
		return writeMap(w, relatedOrFiltered(relations.ConsolePropertyTenancyOverride, group.ConsolePropertyTenancyOverrideMap,
			domain.ConsolePropertyTenancyOverride, related, filter), limit, opts, domain.ConsolePropertyTenancyOverride, env, selected)
	case domain.PropertyTenancyOverride:
		return writeMap(w, relatedOrFiltered(relations.PropertyTenancyOverride, group.PropertyTenancyOverrideMap, domain.PropertyTenancyOverride, related, filter),
			limit, opts, domain.PropertyTenancyOverride, env, selected)
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

// aliasEntryMatches reports whether --filter matches the category's name
// or one of its aliases (case-insensitive substring).
func aliasEntryMatches(e domain.AliasEntry, filter string) bool {
	if strings.Contains(strings.ToLower(e.Name), filter) {
		return true
	}
	for _, a := range e.Aliases {
		if strings.Contains(strings.ToLower(a), filter) {
			return true
		}
	}
	return false
}

func writeAliases(w writer, filter string, limit int, opts output.Options, selected []string) error {
	var (
		entries []domain.AliasEntry
		cats    []domain.Category // the same rows, for the table's column set
	)
	for _, e := range domain.AliasEntries() { // in category order
		if filter != "" && !aliasEntryMatches(e, filter) {
			continue
		}
		c, err := domain.ParseCategory(e.Name)
		if err != nil {
			return err
		}
		entries, cats = append(entries, e), append(cats, c)
	}
	switch opts.Format {
	case output.FormatJSON, output.FormatJSONL, output.FormatYAML:
		return writeEncoded(w, opts, collections.TruncateSlice(entries, limit))
	case output.FormatTable, output.FormatCSV, output.FormatTSV:
		// Alias is a static enum dump — no env-dependent columns,
		// so passing an empty environment is correct.
		return writeSlice(w, cats, limit, opts, domain.Alias, models.Environment{}, selected)
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}
