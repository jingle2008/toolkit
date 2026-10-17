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
// key as a top-level field (GPUNode.NodePool → poolName,
// DedicatedAICluster.TenantID → tenantId, ModelArtifact.ModelName →
// model_name), so wrapping with an injected key would just duplicate.
//
// The SDK reflects on the typed return for OutputSchema — no
// map[string]any leakage.
func flattenGrouped[T models.NamedFilterable](grouped map[string][]T, filter string, limit int) []T {
	return collections.TruncateSlice(flattenSorted(collections.FilterMapOrAll(grouped, normFilter(filter))), limit)
}

// flattenSorted concatenates a grouped map's items in group-key order.
func flattenSorted[T any](grouped map[string][]T) []T {
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0)
	for _, k := range keys {
		out = append(out, grouped[k]...)
	}
	return out
}

// registerTools attaches the read-only category tools to s.server.
// Tool naming convention: list_<category>, mirroring how kubectl /
// gh expose list operations.
func registerTools(s *Server) {
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenants",
		Annotations: readOnly(),
		Description: "List tenants in the configured realm: one per tenancy OCID found in tenancy overrides or tenant metadata, named by its metadata name or, without one, by its OCID. Returns Tenant objects {name, id, is_internal, note, status}; `status` is the tenancy account state recorded by `toolkit tenant sync` (ACTIVE, DELETING, TERMINATED, ALWAYS_FREE_ONLY), omitted when unknown. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListTenants)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_base_models",
		Annotations: readOnly(),
		Description: "List base models loaded from the cluster. Returns BaseModel objects {name, internalName, displayName, vendor, type, version, status, maxTokens, ...}. Tenant-scoped models (ClusterBaseModel CRs with a `tenancy-id` label) are excluded — use list_imported_models for those. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListBaseModels)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_serving_runtimes",
		Annotations: readOnly(),
		Description: "List OME ClusterServingRuntime CRs — the cluster-scoped templates that define how a class of models is served. Returns ServingRuntime objects {name, acceleratorClasses, instanceTypes, image, modelSizeMin, modelSizeMax, cpuLimit, memoryLimit, gpuLimit, disabled, autoSelect}. `acceleratorClasses` (e.g. \"nvidia-h100-8\", family plus tensor-parallel size) and `instanceTypes` (e.g. \"BM.GPU.H100.8\", from the required node affinity) are both optional and may both be absent. `image` and the limits come from the ome-container, preferring spec.engineConfig.runner over spec.containers. `autoSelect` is true when any supportedModelFormats entry sets it. Namespaced ServingRuntime CRs are not included. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListServingRuntimes)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_imported_models",
		Annotations: readOnly(),
		Description: "List tenant-imported models as a flat array (same shape as `list_dacs`); the owning tenant is preserved on each item as `tenantId`. When the tenancy is known to the repo's tenant list (the tenancies in tenancy overrides plus metadata), the item also carries `owner` ({name, id, is_internal, note, status}) and `filter` matches the tenant name; otherwise `owner` is omitted. Sources: (1) ome.io BaseModel CRs across all namespaces (originating namespace on `namespace`); (2) ClusterBaseModel CRs carrying a `tenancy-id` label. Every item carries a non-empty `tenantId` — the label value, or `\"UNKNOWN_TENANCY\"` for orphans (namespaced CRs missing the label, treated as a config error). `namespace` (K8s scope) is orthogonal: empty for cluster-scoped CRs, non-empty for namespaced CRs; a namespaced CR may still carry an arbitrary `tenancy-id` label that the label authoritatively defines. All BaseModel fields (name, displayName, vendor, version, status, storageUri, …) are flattened at the top level. A model copied into several namespaces (its `default` library copy plus copies made into DAC namespaces for serving) is one item: `namespace` is the primary copy (`default` when present) and `copies` lists the other namespaces. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListImportedModels)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_pools",
		Annotations: readOnly(),
		Description: "List GPU pools (self-managed instance pools, self-managed cluster networks, and OKE-managed nodepools). Returns GPUPool objects with live `actualSize` and `status` enriched from OCI's ListInstancePools (matches the TUI). Warnings field is populated when one or more pool sources fail to resolve or enrichment is incomplete. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListGPUPools)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_nodes",
		Annotations: readOnly(),
		Description: "List GPU nodes across all pools as a flat array. The originating pool is preserved on each item as `poolName`. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListGPUNodes)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_gpu_workloads",
		Annotations: readOnly(),
		Description: "List GPU-consuming pods (workloads) across all nodes as a flat array. The hosting node is preserved on each item as `node`. Each item is {name, node, tenantId, namespace, model, runtime, gpus, mode}; model/runtime/mode are best-effort and blank for non-serving GPU pods. Items whose tenancy is known also carry `owner` ({name, id, is_internal, note, status}). Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListGPUWorkloads)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_dacs",
		Annotations: readOnly(),
		Description: "List dedicated AI clusters as a flat array. The owning tenant is preserved on each item as `tenantId`. When the tenancy is known to the repo's tenant list (the tenancies in tenancy overrides plus metadata), the item also carries `owner` ({name, id, is_internal, note, status}) and `filter` matches the tenant name; otherwise `owner` is omitted. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListDACs)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_inference_services",
		Annotations: readOnly(),
		Description: "List OME InferenceService CRs (serving endpoints) across all namespaces as a flat array. Each item is {name, namespace, type, tenantId, model, importedModel, runtime, minReplicas, maxReplicas, ready, age}. `type` is \"dedicated\" (namespace is the dedicated AI cluster ID, matching a `list_dacs` item's `name`) or \"on-demand\" (shared endpoint; namespace is the model name, no `tenantId`). `importedModel` is true when the endpoint serves a tenant-imported model (a namespaced BaseModel ref, or a tenant-labelled ClusterBaseModel, whose name is an OCID resource id). `ready` is the Ready condition status (True/False/Unknown). Items whose tenancy is known also carry `owner` ({name, id, is_internal, note, status}). Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListInferenceServices)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_environments",
		Annotations: readOnly(),
		Description: "List all known toolkit environments (type/region/realm tuples). No env_override needed; returns all envs visible to the configured repo. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListEnvironments)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_service_tenancies",
		Annotations: readOnly(),
		Description: "List service tenancies declared in the toolkit repo. Returns ServiceTenancy objects {name, realm, home_region, regions, environment}. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListServiceTenancies)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_model_artifacts",
		Annotations: readOnly(),
		Description: "List model artifacts (object-storage paths) as a flat array. The owning base-model name is preserved on each item as `model_name`. Supports `limit` (max items after filter, across all groups; 0 = unlimited).",
	}, s.handleListModelArtifacts)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_definitions",
		Annotations: readOnly(),
		Description: "List definitions of the given kind. `kind` must be one of: limit, console_property, property. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListDefinitions)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_tenancy_overrides",
		Annotations: readOnly(),
		Description: "List tenancy-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property. Each item carries the owning tenant via `tenant`. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListTenancyOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_regional_overrides",
		Annotations: readOnly(),
		Description: "List regional-scoped overrides of the given kind. `kind` must be one of: limit, console_property, property. Supports `limit` (max items after filter; 0 = unlimited).",
	}, s.handleListRegionalOverrides)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_related",
		Annotations: readOnly(),
		Description: relatedToolDescription(),
	}, s.handleListRelated)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "list_aliases",
		Annotations: readOnly(),
		Description: "Discovery tool. Lists each registered alias and its canonical category name. Useful for an agent that wants to confirm short codes before calling other tools.",
	}, s.handleListAliases)

	// Read-only, but reports what drain_node started (or the CLI/TUI), so
	// a read-only server can still watch a drain.
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "get_drain_status",
		Annotations: readOnly(),
		Description: "Read-only: report a node's drain as recorded on the Node by drain_node (or the CLI/TUI): phase" +
			" (none/starting/waiting/evicting/done/failed), startedAt, the restarted InferenceServices, the current phase's deadline," +
			" what it still waits on (remaining), and the failure message. Does not advance the drain: if the phase stays" +
			" in progress past its deadline, no process is driving it, so call drain_node again to resume." +
			" Accepts env_type/env_region/env_realm overrides.",
	}, s.handleDrainStatus)

	if s.cfg.MCPReadOnly {
		return
	}
	registerMutationTools(s)
}

// --- Handlers -----------------------------------------------------

// tenantDataset loads env's tenants for resolving tenant-owned rows. A
// load failure degrades to a warning (rows stay keyed by raw tenancy-id
// with no owner) rather than failing the list call.
func (s *Server) tenantDataset(ctx context.Context, env models.Environment) (*models.Dataset, []string) {
	ds, err := resolve.TenantDataset(ctx, s.loader, s.cfg.RepoPath, env)
	if err != nil {
		s.logger.Warnw("tenant resolution unavailable", "surface", "mcp", "error", err)
		return ds, []string{"tenant names unresolved: " + err.Error()}
	}
	return ds, nil
}

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
		return failTool[listResult[models.Tenant]]("load tenants", err)
	}
	return listFlatResult(grp.Tenants, in.Filter, in.Limit, nil)
}

func (s *Server) handleListBaseModels(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.BaseModel], error) {
	items, err := s.loader.LoadBaseModels(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.BaseModel]]("load base models", err)
	}
	return listFlatResult(items, in.Filter, in.Limit, nil)
}

func (s *Server) handleListServingRuntimes(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ServingRuntime], error) {
	items, err := s.loader.LoadServingRuntimes(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ServingRuntime]]("load serving runtimes", err)
	}
	return listFlatResult(items, in.Filter, in.Limit, nil)
}

func (s *Server) handleListImportedModels(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ImportedModel], error) {
	env := s.envFor(in.envOverride)
	grouped, err := s.loader.LoadImportedModels(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return failTool[listResult[models.ImportedModel]]("load imported models", err)
	}
	// Each item carries its own `tenantId`; resolution adds `owner` and
	// lets the filter match the tenant name (the re-keyed group key).
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetImportedModelMap(grouped)
	return jsonResult(flattenGrouped(ds.ImportedModelMap, in.Filter, in.Limit), warnings)
}

func (s *Server) handleListGPUPools(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GPUPool], error) {
	env := s.envFor(in.envOverride)
	items, err := s.loader.LoadGPUPools(ctx, s.cfg.RepoPath, env)
	warnings := warningsFromPartial(err)
	if err != nil && len(warnings) == 0 {
		return failTool[listResult[models.GPUPool]]("load gpu pools", err)
	}
	// The client sees these via listResult.Warnings; the log line is for
	// operators reading cfg.LogFile, matching what mutations already do.
	if len(warnings) > 0 {
		s.logger.Warnw(
			"load gpu pools", "surface", "mcp",
			"phase", "partial", "sources", len(warnings),
			"warnings", strings.Join(warnings, "; "),
		)
	}
	// Enrich ActualSize / Status from OCI's ListInstancePools (same
	// step the TUI runs after load). Degrades to a warning if the K8s
	// or OCI call fails so callers still get Terraform-derived data.
	if err := resolve.EnrichGPUPools(ctx, items, s.cfg.KubeConfig, env); err != nil {
		warnings = append(warnings, "enrichment incomplete: "+err.Error())
		s.logger.Warnw(
			"load gpu pools", "surface", "mcp",
			"phase", "enrichment", "error", err,
		)
	}
	return listFlatResult(items, in.Filter, in.Limit, warnings)
}

func (s *Server) handleListGPUNodes(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GPUNode], error) {
	grouped, err := s.loader.LoadGPUNodesByPool(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.GPUNode]]("load gpu nodes", err)
	}
	// No wrapper: GPUNode.NodePool (JSON `poolName`) already carries
	// the group key. Wrapping would duplicate.
	return jsonResult(flattenGrouped(grouped, in.Filter, in.Limit), nil)
}

func (s *Server) handleListGPUWorkloads(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.GPUWorkload], error) {
	env := s.envFor(in.envOverride)
	grouped, err := s.loader.LoadGPUWorkloadsByNode(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return failTool[listResult[models.GPUWorkload]]("load gpu workloads", err)
	}
	// No wrapper: GPUWorkload.Node (JSON `node`) already carries the
	// group key. Wrapping would duplicate. Owners are resolved in place.
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetGPUWorkloadMap(grouped)
	return jsonResult(flattenGrouped(ds.GPUWorkloadMap, in.Filter, in.Limit), warnings)
}

func (s *Server) handleListDACs(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.DedicatedAICluster], error) {
	env := s.envFor(in.envOverride)
	grouped, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return failTool[listResult[models.DedicatedAICluster]]("load dedicated AI clusters", err)
	}
	// No wrapper: each value carries its raw `tenantId`. Resolution
	// re-keys by tenant name (so the filter matches it) and adds `owner`.
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetDedicatedAIClusterMap(grouped)
	return jsonResult(flattenGrouped(ds.DedicatedAIClusterMap, in.Filter, in.Limit), warnings)
}

func (s *Server) handleListInferenceServices(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.InferenceService], error) {
	env := s.envFor(in.envOverride)
	grouped, err := s.loader.LoadInferenceServices(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return failTool[listResult[models.InferenceService]]("load inference services", err)
	}
	// No wrapper: InferenceService.Namespace (JSON `namespace`) already
	// carries the group key. Wrapping would duplicate. Owners are
	// resolved in place.
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetInferenceServiceMap(grouped)
	return jsonResult(flattenGrouped(ds.InferenceServiceMap, in.Filter, in.Limit), warnings)
}

func (s *Server) handleListEnvironments(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.Environment], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.Environment]]("load dataset", err)
	}
	return listFlatResult(dataset.Environments, in.Filter, in.Limit, nil)
}

func (s *Server) handleListServiceTenancies(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ServiceTenancy], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ServiceTenancy]]("load dataset", err)
	}
	return listFlatResult(dataset.ServiceTenancies, in.Filter, in.Limit, nil)
}

func (s *Server) handleListModelArtifacts(ctx context.Context, req *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listResult[models.ModelArtifact], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[models.ModelArtifact]]("load dataset", err)
	}
	// No wrapper: ModelArtifact.ModelName (JSON `model_name`) already
	// carries the group key.
	return jsonResult(flattenGrouped(dataset.ModelArtifactMap, in.Filter, in.Limit), nil)
}

func (s *Server) handleListDefinitions(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	dataset, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[any]]("load dataset", err)
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
		return failTool[listResult[any]]("list_definitions",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

func (s *Server) handleListTenancyOverrides(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[any]]("load tenancy override group", err)
	}
	switch in.Kind {
	case "limit":
		flat := output.Flatten(collections.FilterMapOrAll(grp.LimitTenancyOverrideMap, normFilter(in.Filter)))
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	case "console_property":
		flat := output.Flatten(collections.FilterMapOrAll(grp.ConsolePropertyTenancyOverrideMap, normFilter(in.Filter)))
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	case "property":
		flat := output.Flatten(collections.FilterMapOrAll(grp.PropertyTenancyOverrideMap, normFilter(in.Filter)))
		return jsonResult(collections.TruncateSlice(toAnySlice(flat), in.Limit), nil)
	default:
		return failTool[listResult[any]]("list_tenancy_overrides",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

func (s *Server) handleListRegionalOverrides(ctx context.Context, req *sdk.CallToolRequest, in kindInput) (*sdk.CallToolResult, listResult[any], error) {
	env := s.envFor(in.envOverride)
	switch in.Kind {
	case "limit":
		items, err := s.loader.LoadLimitRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]]("load limit regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	case "console_property":
		items, err := s.loader.LoadConsolePropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]]("load console property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	case "property":
		items, err := s.loader.LoadPropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return failTool[listResult[any]]("load property regional overrides", err)
		}
		filtered := collections.FilterSlice(items, nil, normFilter(in.Filter), nil)
		return jsonResult(collections.TruncateSlice(toAnySlice(filtered), in.Limit), nil)
	default:
		return failTool[listResult[any]]("list_regional_overrides",
			fmt.Errorf("unknown kind %q (expected: limit, console_property, property)", in.Kind))
	}
}

// aliasItem is the MCP-specific shape for list_aliases: one entry per
// alias. Distinct from `toolkit get alias -o json` which emits one
// entry per category (canonical column set). Kept here for backward
// compatibility with existing agent prompts.
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
