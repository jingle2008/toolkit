package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// --- Grouped handling ---------------------------------------------
//
// flattenGrouped concatenates a grouped map[string][]T into a flat
// []T with deterministic key ordering; applies filter + limit. Used
// by every grouped list_* tool: each model already carries its group
// key as a top-level field (GpuNode.NodePool → poolName,
// DedicatedAICluster.TenantID → tenantId, ModelArtifact.ModelName →
// model_name), so wrapping with an injected key would just duplicate.
//
// The SDK reflects on the typed return for OutputSchema — no
// map[string]any leakage.
func flattenGrouped[T models.NamedFilterable](grouped map[string][]T, filter string, limit int) []T {
	filtered := collections.FilterMapOrAll(grouped, normFilter(filter))
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0)
	for _, k := range keys {
		out = append(out, filtered[k]...)
	}
	return collections.TruncateSlice(out, limit)
}

// registerTools attaches the read-only category tools to s.server.
// Tool naming convention: list_<category>, mirroring how kubectl /
// gh expose list operations.
func registerTools(s *Server) {
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenants",
		Description: "List tenants in the configured realm. Returns Tenant objects {name, ids, is_internal, note}. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListTenants)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_base_models",
		Description: "List base models loaded from the cluster. Returns BaseModel objects {name, internalName, displayName, vendor, type, version, status, maxTokens, ...}. Tenant-scoped models (ClusterBaseModel CRs with a `tenancy-id` label) are excluded — use list_imported_models for those. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListBaseModels)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_imported_models",
		Description: "List tenant-imported models. Sources: (1) ome.io BaseModel CRs across all namespaces (originating namespace on `namespace`); (2) ClusterBaseModel CRs carrying a `tenancy-id` label (label value on `tenantId`). `namespace` and `tenantId` are orthogonal facets: `namespace` is the K8s scope (empty ⇒ cluster-scoped CBM; non-empty ⇒ namespaced BM — this is the authoritative source-kind indicator); `tenantId` is the OCI tenant identifier from the label, which may appear on either source. All BaseModel fields (name, displayName, vendor, version, status, storageUri, …) are flattened at the top level. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListImportedModels)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_pools",
		Description: "List GPU pools (self-managed instance pools, self-managed cluster networks, and OKE-managed nodepools). Returns GpuPool objects with live `actualSize` and `status` enriched from OCI's ListInstancePools (matches the TUI). Warnings field is populated when one or more pool sources fail to resolve or enrichment is incomplete. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListGpuPools)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_nodes",
		Description: "List GPU nodes across all pools as a flat array. The originating pool is preserved on each item as `poolName`. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListGpuNodes)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_dacs",
		Description: "List dedicated AI clusters as a flat array. The owning tenant is preserved on each item as `tenantId`. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListDACs)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_environments",
		Description: "List all known toolkit environments (type/region/realm tuples). No env_override needed; returns all envs visible to the configured repo. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListEnvironments)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_service_tenancies",
		Description: "List service tenancies declared in the toolkit repo. Returns ServiceTenancy objects {name, realm, home_region, regions, environment}. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListServiceTenancies)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_model_artifacts",
		Description: "List model artifacts (object-storage paths) as a flat array. The owning base-model name is preserved on each item as `model_name`. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListModelArtifacts)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_definitions",
		Description: "List definitions of the given kind. `kind` must be one of: limit, console_property, property. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListDefinitions)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenancy_overrides",
		Description: "List tenancy-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property. Each item carries the owning tenant via `tenant`. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListTenancyOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_regional_overrides",
		Description: "List regional-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListRegionalOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_aliases",
		Description: "Discovery tool. Lists every category alias and its canonical category name. Useful for an agent that wants to confirm short codes before calling other tools.",
	}, s.handleListAliases)

	registerMutationTools(s)
}

// --- Handlers -----------------------------------------------------

// listFlatResult applies the shared filter + limit + JSON-envelope
// step to an already-loaded slice. Captures the trailing pattern of
// every list_* tool that returns []T directly.
func listFlatResult[T models.NamedFilterable](items []T, filter string, limit int, warnings []string) (*sdk.CallToolResult, listResult[T], error) {
	filtered := collections.FilterSlice(items, nil, normFilter(filter), nil)
	return jsonResult(collections.TruncateSlice(filtered, limit), warnings)
}

// toAnySlice copies items into []any. Used by the polymorphic
// handlers (list_definitions / list_tenancy_overrides /
// list_regional_overrides) whose switch-on-kind branches each
// produce a different element type — the function's single Out
// must therefore be `listResult[any]`. The schema for these tools
// is necessarily generic; the StructuredContent still carries
// the concrete typed objects.
func toAnySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, x := range items {
		out[i] = x
	}
	return out
}

func (s *Server) handleListTenants(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.Tenant], error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.Tenant]](ctx, req, "load tenants", err)
	}
	return listFlatResult(grp.Tenants, in.Filter, in.Limit, nil)
}

func (s *Server) handleListBaseModels(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.BaseModel], error) {
	items, err := s.loader.LoadBaseModels(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.BaseModel]](ctx, req, "load base models", err)
	}
	return listFlatResult(items, in.Filter, in.Limit, nil)
}

func (s *Server) handleListImportedModels(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ImportedModel], error) {
	items, err := s.loader.LoadImportedModels(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ImportedModel]](ctx, req, "load imported models", err)
	}
	return listFlatResult(items, in.Filter, in.Limit, nil)
}

func (s *Server) handleListGpuPools(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GpuPool], error) {
	env := s.envFor(in.envOverride)
	items, err := s.loader.LoadGpuPools(ctx, s.cfg.RepoPath, env)
	warnings := warningsFromPartial(err)
	if err != nil && len(warnings) == 0 {
		return failTool[listResult[models.GpuPool]](ctx, req, "load gpu pools", err)
	}
	if len(warnings) > 0 {
		notify(ctx, req.Session, "warning",
			fmt.Sprintf("load gpu pools: %d source(s) returned partial results: %s",
				len(warnings), strings.Join(warnings, "; ")))
	}
	// Enrich ActualSize / Status from OCI's ListInstancePools (same
	// step the TUI runs after load). Degrades to a warning if the K8s
	// or OCI call fails so callers still get Terraform-derived data.
	if msg := resolve.EnrichGpuPools(ctx, items, s.cfg.KubeConfig, env); msg != "" {
		warnings = append(warnings, "enrichment incomplete: "+msg)
		notify(ctx, req.Session, "warning", "gpu pool enrichment incomplete: "+msg)
	}
	return listFlatResult(items, in.Filter, in.Limit, warnings)
}

func (s *Server) handleListGpuNodes(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GpuNode], error) {
	grouped, err := s.loader.LoadGpuNodes(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.GpuNode]](ctx, req, "load gpu nodes", err)
	}
	// No wrapper: GpuNode.NodePool (JSON `poolName`) already carries
	// the group key. Wrapping would duplicate.
	return jsonResult(flattenGrouped(grouped, in.Filter, in.Limit), nil)
}

func (s *Server) handleListDACs(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.DedicatedAICluster], error) {
	grouped, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.DedicatedAICluster]](ctx, req, "load dedicated AI clusters", err)
	}
	// No wrapper: the loader keys this map by dac.TenantID
	// (internal/infra/k8s/dac.go:157), which is already the flat
	// `tenantId` field on each value. Wrapping would duplicate.
	return jsonResult(flattenGrouped(grouped, in.Filter, in.Limit), nil)
}

func (s *Server) handleListEnvironments(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.Environment], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.Environment]](ctx, req, "load dataset", err)
	}
	return listFlatResult(dataset.Environments, in.Filter, in.Limit, nil)
}

func (s *Server) handleListServiceTenancies(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ServiceTenancy], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ServiceTenancy]](ctx, req, "load dataset", err)
	}
	return listFlatResult(dataset.ServiceTenancies, in.Filter, in.Limit, nil)
}

func (s *Server) handleListModelArtifacts(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ModelArtifact], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ModelArtifact]](ctx, req, "load dataset", err)
	}
	// No wrapper: ModelArtifact.ModelName (JSON `model_name`) already
	// carries the group key.
	return jsonResult(flattenGrouped(dataset.ModelArtifactMap, in.Filter, in.Limit), nil)
}

func (s *Server) handleListDefinitions(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[any]](ctx, req, "load dataset", err)
	}
	switch in.Kind {
	case "limit":
		items := collections.FilterSlice(dataset.LimitDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(items), in.Limit), nil)
	case "console_property":
		items := collections.FilterSlice(dataset.ConsolePropertyDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(items), in.Limit), nil)
	case "property":
		items := collections.FilterSlice(dataset.PropertyDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(items), in.Limit), nil)
	default:
		return failTool[listResult[any]](ctx, req, "list_definitions",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

func (s *Server) handleListTenancyOverrides(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[any]](ctx, req, "load tenancy override group", err)
	}
	switch in.Kind {
	case "limit":
		flat := output.FlattenWithKey(collections.FilterMapOrAll(grp.LimitTenancyOverrideMap, normFilter(in.Filter)), "tenant")
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	case "console_property":
		flat := output.FlattenWithKey(collections.FilterMapOrAll(grp.ConsolePropertyTenancyOverrideMap, normFilter(in.Filter)), "tenant")
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	case "property":
		flat := output.FlattenWithKey(collections.FilterMapOrAll(grp.PropertyTenancyOverrideMap, normFilter(in.Filter)), "tenant")
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	default:
		return failTool[listResult[any]](ctx, req, "list_tenancy_overrides",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

func (s *Server) handleListRegionalOverrides(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	env := s.envFor(in.envOverride)
	switch in.Kind {
	case "limit":
		items, err := s.loader.LoadLimitRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]](ctx, req, "load limit regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	case "console_property":
		items, err := s.loader.LoadConsolePropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]](ctx, req, "load console property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	case "property":
		items, err := s.loader.LoadPropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]](ctx, req, "load property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	default:
		return failTool[listResult[any]](ctx, req, "list_regional_overrides",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

// aliasItem matches the CLI's `toolkit get alias` shape so agents can
// trust both surfaces.
type aliasItem struct {
	Alias    string `json:"alias"`
	Category string `json:"category"`
}

// noInput is used by tools that take no arguments; the SDK requires
// a concrete type even when the schema is empty.
type noInput struct{}

func (s *Server) handleListAliases(_ context.Context, _ *sdk.CallToolRequest, _ noInput) (*sdk.CallToolResult, listResult[aliasItem], error) {
	items := make([]aliasItem, 0, len(domain.Aliases))
	for _, a := range domain.Aliases {
		cat, err := domain.ParseCategory(a)
		if err != nil {
			continue
		}
		items = append(items, aliasItem{Alias: a, Category: cat.String()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Alias < items[j].Alias })
	return jsonResult(items, nil)
}
