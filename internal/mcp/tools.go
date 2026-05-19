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
	"github.com/jingle2008/toolkit/pkg/models"
)

// --- Grouped item wrappers ---------------------------------------
//
// MCP clients (and the SDK's reflection-based OutputSchema generator)
// see these embedded shapes as a flat object with one extra field
// holding the group key. That preserves the typed-schema benefit
// the refactor was meant to deliver — previously the grouped handlers
// returned []map[string]any (via output.FlattenWithKey), which is
// opaque to the schema generator and shows up as a generic object.
// Wire format is unchanged.

type gpuNodeWithPool struct {
	Pool string `json:"pool"`
	models.GpuNode
}

type dacWithTenant struct {
	Tenant string `json:"tenant"`
	models.DedicatedAICluster
}

type modelArtifactWithModel struct {
	Model string `json:"model"`
	models.ModelArtifact
}

// flattenGroupedTyped is the typed counterpart to output.FlattenWithKey.
// It applies the shared filter to the grouped map, sorts keys for
// deterministic output, and wraps each (key, item) pair into W via
// mkWrap — preserving Go type info through to the SDK schema generator.
//
// Used by the three grouped read tools (list_gpu_nodes, list_dacs,
// list_model_artifacts). The CLI's get path still uses
// output.FlattenWithKey because CLI consumers don't care about
// OutputSchema; only the MCP wire shape benefits from typing here.
func flattenGroupedTyped[T models.NamedFilterable, W any](
	grouped map[string][]T,
	filter string,
	mkWrap func(key string, item T) W,
) []W {
	filtered := collections.FilterMapOrAll(grouped, normFilter(filter))
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]W, 0)
	for _, k := range keys {
		for _, item := range filtered[k] {
			out = append(out, mkWrap(k, item))
		}
	}
	return out
}

// registerTools attaches the read-only category tools to s.server.
// Tool naming convention: list_<category>, mirroring how kubectl /
// gh expose list operations.
func registerTools(s *Server) {
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenants",
		Description: "List tenants in the configured realm. Returns Tenant objects {name, ids, is_internal, note}.",
	}, s.handleListTenants)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_base_models",
		Description: "List base models loaded from the cluster. Returns BaseModel objects {name, internalName, displayName, vendor, type, version, status, maxTokens, ...}.",
	}, s.handleListBaseModels)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_pools",
		Description: "List GPU pools (self-managed instance pools, self-managed cluster networks, and OKE-managed nodepools). Returns GpuPool objects. Warnings field is populated when one or more pool sources fail to resolve.",
	}, s.handleListGpuPools)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_nodes",
		Description: "List GPU nodes across all pools as a flat array. Each item carries the originating pool via NodePool plus a `pool` field added by the server.",
	}, s.handleListGpuNodes)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_dacs",
		Description: "List dedicated AI clusters as a flat array. Each item carries the owning tenant via Owner.Name plus a `tenant` field added by the server.",
	}, s.handleListDACs)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_environments",
		Description: "List all known toolkit environments (type/region/realm tuples). No env_override needed; returns all envs visible to the configured repo.",
	}, s.handleListEnvironments)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_service_tenancies",
		Description: "List service tenancies declared in the toolkit repo. Returns ServiceTenancy objects {name, realm, home_region, regions, environment}.",
	}, s.handleListServiceTenancies)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_model_artifacts",
		Description: "List model artifacts (object-storage paths) as a flat array. Each item carries the owning base-model name via `model`.",
	}, s.handleListModelArtifacts)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_definitions",
		Description: "List definitions of the given kind. `kind` must be one of: limit, console_property, property.",
	}, s.handleListDefinitions)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenancy_overrides",
		Description: "List tenancy-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property. Each item carries the owning tenant via `tenant`.",
	}, s.handleListTenancyOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_regional_overrides",
		Description: "List regional-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property.",
	}, s.handleListRegionalOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_aliases",
		Description: "Discovery tool. Lists every category alias and its canonical category name. Useful for an agent that wants to confirm short codes before calling other tools.",
	}, s.handleListAliases)

	registerMutationTools(s)
}

// --- Handlers -----------------------------------------------------

// listFlatResult applies the shared filter + JSON-envelope step to an
// already-loaded slice. Captures the trailing pattern of every list_*
// tool that returns []T directly.
func listFlatResult[T models.NamedFilterable](items []T, filter string, warnings []string) (*sdk.CallToolResult, listResult[T], error) {
	return jsonResult(collections.FilterSlice(items, nil, normFilter(filter), nil), warnings)
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
	return listFlatResult(grp.Tenants, in.Filter, nil)
}

func (s *Server) handleListBaseModels(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.BaseModel], error) {
	items, err := s.loader.LoadBaseModels(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.BaseModel]](ctx, req, "load base models", err)
	}
	return listFlatResult(items, in.Filter, nil)
}

func (s *Server) handleListGpuPools(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GpuPool], error) {
	items, err := s.loader.LoadGpuPools(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	warnings := warningsFromPartial(err)
	if err != nil && len(warnings) == 0 {
		return failTool[listResult[models.GpuPool]](ctx, req, "load gpu pools", err)
	}
	if len(warnings) > 0 {
		notify(ctx, req.Session, "warning",
			fmt.Sprintf("load gpu pools: %d source(s) returned partial results: %s",
				len(warnings), strings.Join(warnings, "; ")))
	}
	return listFlatResult(items, in.Filter, warnings)
}

func (s *Server) handleListGpuNodes(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[gpuNodeWithPool], error) {
	grouped, err := s.loader.LoadGpuNodes(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[gpuNodeWithPool]](ctx, req, "load gpu nodes", err)
	}
	flat := flattenGroupedTyped(grouped, in.Filter, func(pool string, n models.GpuNode) gpuNodeWithPool {
		return gpuNodeWithPool{Pool: pool, GpuNode: n}
	})
	return jsonResult(flat, nil)
}

func (s *Server) handleListDACs(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[dacWithTenant], error) {
	grouped, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[dacWithTenant]](ctx, req, "load dedicated AI clusters", err)
	}
	flat := flattenGroupedTyped(grouped, in.Filter, func(tenant string, d models.DedicatedAICluster) dacWithTenant {
		return dacWithTenant{Tenant: tenant, DedicatedAICluster: d}
	})
	return jsonResult(flat, nil)
}

func (s *Server) handleListEnvironments(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.Environment], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.Environment]](ctx, req, "load dataset", err)
	}
	return listFlatResult(dataset.Environments, in.Filter, nil)
}

func (s *Server) handleListServiceTenancies(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ServiceTenancy], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ServiceTenancy]](ctx, req, "load dataset", err)
	}
	return listFlatResult(dataset.ServiceTenancies, in.Filter, nil)
}

func (s *Server) handleListModelArtifacts(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[modelArtifactWithModel], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[modelArtifactWithModel]](ctx, req, "load dataset", err)
	}
	flat := flattenGroupedTyped(dataset.ModelArtifactMap, in.Filter, func(model string, a models.ModelArtifact) modelArtifactWithModel {
		return modelArtifactWithModel{Model: model, ModelArtifact: a}
	})
	return jsonResult(flat, nil)
}

func (s *Server) handleListDefinitions(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[any]](ctx, req, "load dataset", err)
	}
	switch in.Kind {
	case "limit":
		items := collections.FilterSlice(dataset.LimitDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(toAnySlice(items), nil)
	case "console_property":
		items := collections.FilterSlice(dataset.ConsolePropertyDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(toAnySlice(items), nil)
	case "property":
		items := collections.FilterSlice(dataset.PropertyDefinitionGroup.Values, nil, normFilter(in.Filter), nil)
		return jsonResult(toAnySlice(items), nil)
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
		return jsonResult(toAnySlice(flat), nil)
	case "console_property":
		flat := output.FlattenWithKey(collections.FilterMapOrAll(grp.ConsolePropertyTenancyOverrideMap, normFilter(in.Filter)), "tenant")
		return jsonResult(toAnySlice(flat), nil)
	case "property":
		flat := output.FlattenWithKey(collections.FilterMapOrAll(grp.PropertyTenancyOverrideMap, normFilter(in.Filter)), "tenant")
		return jsonResult(toAnySlice(flat), nil)
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
		return jsonResult(toAnySlice(filtered), nil)
	case "console_property":
		items, err := s.loader.LoadConsolePropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]](ctx, req, "load console property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(toAnySlice(filtered), nil)
	case "property":
		items, err := s.loader.LoadPropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]](ctx, req, "load property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(toAnySlice(filtered), nil)
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
