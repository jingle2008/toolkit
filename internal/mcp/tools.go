package mcp

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

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
}

// --- Handlers -----------------------------------------------------

func (s *Server) handleListTenants(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load tenants: %w", err)
	}
	items := collections.FilterSlice(grp.Tenants, nil, normFilter(in.Filter), nil)
	return jsonResult(items, nil)
}

func (s *Server) handleListBaseModels(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	items, err := s.loader.LoadBaseModels(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load base models: %w", err)
	}
	filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
	return jsonResult(filtered, nil)
}

func (s *Server) handleListGpuPools(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	items, err := s.loader.LoadGpuPools(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	warnings := warningsFromPartial(err)
	if err != nil && len(warnings) == 0 {
		return nil, struct{}{}, fmt.Errorf("load gpu pools: %w", err)
	}
	filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
	return jsonResult(filtered, warnings)
}

func (s *Server) handleListGpuNodes(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	grouped, err := s.loader.LoadGpuNodes(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load gpu nodes: %w", err)
	}
	flat := output.FlattenWithKey(filterMap(grouped, normFilter(in.Filter)), "pool")
	return jsonResult(flat, nil)
}

func (s *Server) handleListDACs(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	grouped, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load dedicated AI clusters: %w", err)
	}
	flat := output.FlattenWithKey(filterMap(grouped, normFilter(in.Filter)), "tenant")
	return jsonResult(flat, nil)
}

func (s *Server) handleListEnvironments(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load dataset: %w", err)
	}
	items := collections.FilterSlice(dataset.Environments, nil, normFilter(in.Filter), nil)
	return jsonResult(items, nil)
}

func (s *Server) handleListServiceTenancies(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load dataset: %w", err)
	}
	items := collections.FilterSlice(dataset.ServiceTenancies, nil, normFilter(in.Filter), nil)
	return jsonResult(items, nil)
}

func (s *Server) handleListModelArtifacts(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, struct{}, error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load dataset: %w", err)
	}
	flat := output.FlattenWithKey(filterMap(dataset.ModelArtifactMap, normFilter(in.Filter)), "model")
	return jsonResult(flat, nil)
}

func (s *Server) handleListDefinitions(ctx context.Context, _ *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, struct{}, error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load dataset: %w", err)
	}
	f := normFilter(in.Filter)
	switch in.Kind {
	case "limit":
		return jsonResult(collections.FilterSlice(dataset.LimitDefinitionGroup.Values, nil, f, nil), nil)
	case "console_property":
		return jsonResult(collections.FilterSlice(dataset.ConsolePropertyDefinitionGroup.Values, nil, f, nil), nil)
	case "property":
		return jsonResult(collections.FilterSlice(dataset.PropertyDefinitionGroup.Values, nil, f, nil), nil)
	default:
		return nil, struct{}{}, fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind)
	}
}

func (s *Server) handleListTenancyOverrides(ctx context.Context, _ *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, struct{}, error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return nil, struct{}{}, fmt.Errorf("load tenancy override group: %w", err)
	}
	f := normFilter(in.Filter)
	switch in.Kind {
	case "limit":
		return jsonResult(output.FlattenWithKey(filterMap(grp.LimitTenancyOverrideMap, f), "tenant"), nil)
	case "console_property":
		return jsonResult(output.FlattenWithKey(filterMap(grp.ConsolePropertyTenancyOverrideMap, f), "tenant"), nil)
	case "property":
		return jsonResult(output.FlattenWithKey(filterMap(grp.PropertyTenancyOverrideMap, f), "tenant"), nil)
	default:
		return nil, struct{}{}, fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind)
	}
}

func (s *Server) handleListRegionalOverrides(ctx context.Context, _ *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, struct{}, error) {
	env := s.envFor(in.envOverride)
	f := normFilter(in.Filter)
	switch in.Kind {
	case "limit":
		items, err := s.loader.LoadLimitRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, struct{}{}, fmt.Errorf("load limit regional overrides: %w", err)
		}
		return jsonResult(collections.FilterSlice(items, nil, f, nil), nil)
	case "console_property":
		items, err := s.loader.LoadConsolePropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, struct{}{}, fmt.Errorf("load console property regional overrides: %w", err)
		}
		return jsonResult(collections.FilterSlice(items, nil, f, nil), nil)
	case "property":
		items, err := s.loader.LoadPropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, struct{}{}, fmt.Errorf("load property regional overrides: %w", err)
		}
		return jsonResult(collections.FilterSlice(items, nil, f, nil), nil)
	default:
		return nil, struct{}{}, fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind)
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

func (s *Server) handleListAliases(_ context.Context, _ *sdk.CallToolRequest, _ noInput) (*sdk.CallToolResult, struct{}, error) {
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

// filterMap mirrors the helper in internal/cli/get.go.
func filterMap[T models.NamedFilterable](grouped map[string][]T, filter string) map[string][]T {
	if filter == "" {
		return grouped
	}
	return collections.FilterMap(grouped, nil, nil, filter, nil)
}
