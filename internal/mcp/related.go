package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

// relatedInput selects the rows of Category related to one parent row.
type relatedInput struct {
	Category       string `json:"category" jsonschema:"the category to list, as a name or alias (e.g. gpuworkload / gw, inferenceservice / is, dedicatedaicluster / dac); see list_aliases"`
	ParentCategory string `json:"parent_category" jsonschema:"the parent row's category, as a name or alias (e.g. dac, tenant, servingruntime); must be related to category — the tool description lists the relations"`
	ParentName     string `json:"parent_name" jsonschema:"the parent row's name, matched exactly (case-sensitive): the name field of its list_* item, e.g. a DAC's name, a tenant's name, a runtime's name"`
	Faulty         bool   `json:"faulty,omitempty" jsonschema:"only faulty rows, as faulty=true on the list tools and the TUI's Ctrl+Z inside a drill-in; for gpunode, gpuworkload, dedicatedaicluster and inferenceservice. Loads every category, so it is slower."`
	listInput
}

// relatedToolDescription documents list_related, listing every relation
// from the domain registry so an agent can see the valid pairs.
func relatedToolDescription() string {
	byChild := make(map[domain.Category][]string)
	var order []domain.Category
	for _, r := range domain.Relations() {
		if _, seen := byChild[r.Child]; !seen {
			order = append(order, r.Child)
		}
		byChild[r.Child] = append(byChild[r.Child], r.Parent.String())
	}
	lines := make([]string, len(order))
	for i, c := range order {
		lines[i] = fmt.Sprintf("%s ← %s", c, strings.Join(byChild[c], ", "))
	}
	return "List the rows of `category` related to one parent row (`parent_category` + `parent_name`), " +
		"through the same relations as the TUI's drill-in and `toolkit get --related-to`. " +
		"Returns the same item objects as the category's list_* tool. " +
		"Relations (category ← parents): " + strings.Join(lines, "; ") + ". " +
		"For example a DAC's pods (category gpuworkload, parent dac), the endpoints serving a model " +
		"(inferenceservice, basemodel or importedmodel), or what uses a runtime (inferenceservice or gpuworkload, " +
		"parent servingruntime — pods also cover endpoints whose runtime OME selected at deploy time). " +
		"Tenant parents match rows by their resolved owner. `parent_name` must match exactly; an empty result " +
		"carries a warning. Supports `filter` and `limit` like the list_* tools."
}

func (s *Server) handleListRelated(ctx context.Context, _ *sdk.CallToolRequest, in relatedInput) (*sdk.CallToolResult, listResult[any], error) {
	child, err := domain.ParseCategory(in.Category)
	if err != nil {
		return failTool[listResult[any]]("list_related", fmt.Errorf("category: %w", err))
	}
	parent, err := domain.ParseCategory(in.ParentCategory)
	if err != nil {
		return failTool[listResult[any]]("list_related", fmt.Errorf("parent_category: %w", err))
	}
	if in.ParentName == "" {
		return failTool[listResult[any]]("list_related", fmt.Errorf("parent_name is required"))
	}
	if !parent.IsScopeOf(child) {
		names := make([]string, 0)
		for _, c := range child.ScopedBy() {
			names = append(names, c.String())
		}
		if len(names) == 0 {
			return failTool[listResult[any]]("list_related", fmt.Errorf("%s rows are not related to any other category", child))
		}
		return failTool[listResult[any]]("list_related",
			fmt.Errorf("%s is not related to %s; valid parent_category values: %s", parent, child, strings.Join(names, ", ")))
	}

	scope := &domain.Scope{Category: parent, Name: in.ParentName}
	related := s.relatedItems
	if in.Faulty {
		if err := relations.FaultyCategoryErr(child); err != nil {
			return failTool[listResult[any]]("list_related", fmt.Errorf("faulty: %w", err))
		}
		related = s.relatedFaultyItems
	}
	env, filter := s.envFor(in.envOverride), normFilter(in.Filter)
	items, warnings, err := related(ctx, child, scope, env, filter)
	if err != nil {
		return failTool[listResult[any]]("list_related", err)
	}
	if len(items) == 0 {
		warnings = append(warnings, s.noneRelatedWarning(ctx, child, scope, env, filter, in.Faulty))
	}
	return jsonResult(collections.TruncateSlice(items, in.Limit), warnings)
}

// noneRelatedWarning says why list_related came back empty. Only when no
// row at all relates to the parent does it point at parent_name, which
// may be mistyped; when the filter or faulty-only mode hid the related
// rows, it says that instead.
func (s *Server) noneRelatedWarning(ctx context.Context, child domain.Category, scope *domain.Scope, env models.Environment, filter string, faulty bool) string {
	if filter != "" || faulty {
		if all, _, err := s.relatedItems(ctx, child, scope, env, ""); err == nil && len(all) > 0 {
			return fmt.Sprintf("%d %s rows are related to %s %q, but none match the filter or faulty-only mode",
				len(all), child, scope.Category, scope.Name)
		}
	}
	return fmt.Sprintf("no %s rows related to %s %q (parent_name must match exactly)", child, scope.Category, scope.Name)
}

// relatedItems loads child — resolving tenant owners where a Tenant scope
// or the item's owner field needs them, as the list_* tools do — and
// keeps the rows scope relates to. Only categories with relations reach
// here (handleListRelated validated the pair).
//
//nolint:cyclop // one case per scoped category, mirroring the list_* handlers
func (s *Server) relatedItems(ctx context.Context, child domain.Category, scope *domain.Scope, env models.Environment, filter string) ([]any, []string, error) {
	switch child {
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		grp, err := s.tenancyGroup(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		switch child {
		case domain.LimitTenancyOverride:
			return flattenRelated(relations.LimitTenancyOverride, grp.LimitTenancyOverrideMap, scope, filter), nil, nil
		case domain.ConsolePropertyTenancyOverride:
			return flattenRelated(relations.ConsolePropertyTenancyOverride, grp.ConsolePropertyTenancyOverrideMap, scope, filter), nil, nil
		default:
			return flattenRelated(relations.PropertyTenancyOverride, grp.PropertyTenancyOverrideMap, scope, filter), nil, nil
		}
	case domain.LimitRegionalOverride:
		items, err := s.loader.LoadLimitRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, nil, fmt.Errorf("load limit regional overrides: %w", err)
		}
		return toAnySlice(collections.FilterSlice(items, nil, filter, relations.LimitRegionalOverride.Keep(scope))), nil, nil
	case domain.ConsolePropertyRegionalOverride:
		items, err := s.loader.LoadConsolePropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, nil, fmt.Errorf("load console property regional overrides: %w", err)
		}
		return toAnySlice(collections.FilterSlice(items, nil, filter, relations.ConsolePropertyRegionalOverride.Keep(scope))), nil, nil
	case domain.PropertyRegionalOverride:
		items, err := s.loader.LoadPropertyRegionalOverrides(ctx, s.cfg.RepoPath, env)
		if err != nil {
			return nil, nil, fmt.Errorf("load property regional overrides: %w", err)
		}
		return toAnySlice(collections.FilterSlice(items, nil, filter, relations.PropertyRegionalOverride.Keep(scope))), nil, nil
	case domain.GPUNode:
		grouped, err := s.gpuNodes(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		return flattenRelated(relations.GPUNode, grouped, scope, filter), nil, nil
	case domain.ImportedModel:
		grouped, warnings, err := s.importedModels(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		return flattenRelated(relations.ImportedModel, grouped, scope, filter), warnings, nil
	case domain.GPUWorkload:
		grouped, warnings, err := s.gpuWorkloads(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		return flattenRelated(relations.GPUWorkload, grouped, scope, filter), warnings, nil
	case domain.DedicatedAICluster:
		grouped, warnings, err := s.dacs(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		return flattenRelated(relations.DedicatedAICluster, grouped, scope, filter), warnings, nil
	case domain.InferenceService:
		grouped, warnings, err := s.inferenceServices(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		return flattenRelated(relations.InferenceService, grouped, scope, filter), warnings, nil
	default:
		return nil, nil, fmt.Errorf("%s has no related rows", child)
	}
}

// relatedFaultyItems is relatedItems for faulty=true: of the rows scope
// relates to, only the faulty ones (relations.FaultyRows), from the full
// load deciding them takes. Only the faulty categories with parents reach
// here (handleListRelated validated the pair and the category).
func (s *Server) relatedFaultyItems(ctx context.Context, child domain.Category, scope *domain.Scope, env models.Environment, filter string) ([]any, []string, error) {
	ds, warnings, err := mcpFaultyDatasetFn(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, child)
	if err != nil {
		return nil, nil, err
	}
	switch child {
	case domain.GPUNode:
		return flattenRelated(relations.GPUNode, relations.FaultyMap(ds, child, ds.GPUNodeMap), scope, filter), warnings, nil
	case domain.GPUWorkload:
		return flattenRelated(relations.GPUWorkload, relations.FaultyMap(ds, child, ds.GPUWorkloadMap), scope, filter), warnings, nil
	case domain.DedicatedAICluster:
		return flattenRelated(relations.DedicatedAICluster, relations.FaultyMap(ds, child, ds.DedicatedAIClusterMap), scope, filter), warnings, nil
	case domain.InferenceService:
		return flattenRelated(relations.InferenceService, relations.FaultyMap(ds, child, ds.InferenceServiceMap), scope, filter), warnings, nil
	default:
		return nil, nil, fmt.Errorf("%s has no faulty related rows", child)
	}
}

// flattenRelated keeps the rows of grouped that scope relates to (and
// filter matches), flattened in group-key order like flattenGrouped.
func flattenRelated[T models.NamedFilterable](keys relations.Grouped[T], grouped map[string][]T, scope *domain.Scope, filter string) []any {
	return toAnySlice(flattenSorted(keys.Filter(grouped, scope, filter, nil)))
}
