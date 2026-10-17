package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

type getItemInput struct {
	Category string `json:"category" jsonschema:"the category, by name or alias (see list_aliases), e.g. dac, gpunode, inferenceservice"`
	Name     string `json:"name" jsonschema:"the row's name, exactly as the list tool shows it (case-sensitive)"`
	envOverride
}

const getItemDescription = "Get the rows of one category named exactly `name` — the full objects the matching list_* tool returns," +
	" without filtering a whole list. `items` usually holds one row; several only when rows in different groups share the name" +
	" (an override's name across tenants, an endpoint's across namespaces), each telling its group apart by its own fields." +
	" No row of that name is an error. Accepts env_type/env_region/env_realm overrides."

func (s *Server) handleGetItem(ctx context.Context, req *sdk.CallToolRequest, in getItemInput) (*sdk.CallToolResult, listResult[any], error) {
	cat, err := domain.ParseCategory(in.Category)
	if err != nil {
		return failTool[listResult[any]]("get_item", fmt.Errorf("category: %w", err))
	}
	if in.Name == "" {
		return failTool[listResult[any]]("get_item", fmt.Errorf("name is required"))
	}
	items, warnings, err := s.listAll(ctx, req, cat, in.envOverride)
	if err != nil {
		return failTool[listResult[any]]("get_item", err)
	}
	var found []any
	for _, it := range items {
		if n, ok := it.(models.NamedItem); ok && n.GetName() == in.Name {
			found = append(found, it)
		}
	}
	if len(found) == 0 {
		return failTool[listResult[any]]("get_item",
			fmt.Errorf("no %s named %q (the name must match exactly; list tools' filter matches loosely)", cat, in.Name))
	}
	return jsonResult(found, warnings)
}

// listAll is every row of cat, as its list_* tool returns it unfiltered:
// get_item reuses the list handlers so a row looks the same either way.
//
//nolint:cyclop // one case per category, each a list tool
func (s *Server) listAll(ctx context.Context, req *sdk.CallToolRequest, cat domain.Category, env envOverride) ([]any, []string, error) {
	li := listInput{envOverride: env}
	fi := faultyListInput{listInput: li}
	kind := func(k string) kindInput { return kindInput{Kind: k, listInput: li} }
	switch cat {
	case domain.Tenant:
		return anyItems(s.handleListTenants(ctx, req, fi))
	case domain.BaseModel:
		return anyItems(s.handleListBaseModels(ctx, req, fi))
	case domain.ServingRuntime:
		return anyItems(s.handleListServingRuntimes(ctx, req, fi))
	case domain.ImportedModel:
		return anyItems(s.handleListImportedModels(ctx, req, li))
	case domain.GPUPool:
		return anyItems(s.handleListGPUPools(ctx, req, fi))
	case domain.GPUNode:
		return anyItems(s.handleListGPUNodes(ctx, req, fi))
	case domain.GPUWorkload:
		return anyItems(s.handleListGPUWorkloads(ctx, req, fi))
	case domain.DedicatedAICluster:
		return anyItems(s.handleListDACs(ctx, req, fi))
	case domain.InferenceService:
		return anyItems(s.handleListInferenceServices(ctx, req, fi))
	case domain.Environment:
		return anyItems(s.handleListEnvironments(ctx, req, li))
	case domain.ServiceTenancy:
		return anyItems(s.handleListServiceTenancies(ctx, req, li))
	case domain.ModelArtifact:
		return anyItems(s.handleListModelArtifacts(ctx, req, li))
	case domain.LimitDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("limit")))
	case domain.ConsolePropertyDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("console_property")))
	case domain.PropertyDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("property")))
	case domain.LimitTenancyOverride:
		return anyItems(s.handleListTenancyOverrides(ctx, req, kind("limit")))
	case domain.ConsolePropertyTenancyOverride:
		return anyItems(s.handleListTenancyOverrides(ctx, req, kind("console_property")))
	case domain.PropertyTenancyOverride:
		return anyItems(s.handleListTenancyOverrides(ctx, req, kind("property")))
	case domain.LimitRegionalOverride:
		return anyItems(s.handleListRegionalOverrides(ctx, req, kind("limit")))
	case domain.ConsolePropertyRegionalOverride:
		return anyItems(s.handleListRegionalOverrides(ctx, req, kind("console_property")))
	case domain.PropertyRegionalOverride:
		return anyItems(s.handleListRegionalOverrides(ctx, req, kind("property")))
	default:
		return nil, nil, fmt.Errorf("%s has no rows to get by name", cat)
	}
}

// anyItems adapts a list handler's result to listAll's.
func anyItems[T any](_ *sdk.CallToolResult, r listResult[T], err error) ([]any, []string, error) {
	if err != nil {
		return nil, nil, err
	}
	return toAnySlice(r.Items), r.Warnings, nil
}
