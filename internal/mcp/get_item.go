package mcp

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

type getItemInput struct {
	Category string `json:"category" jsonschema:"the category, by name or alias (see list_aliases), e.g. dac, gpunode, inferenceservice"`
	Name     string `json:"name" jsonschema:"the row's name, exactly as the list tool shows it (case-sensitive)"`
	envOverride
}

const getItemDescription = "Get the one row of a category named exactly `name` — the full object the matching list_* tool returns," +
	" without filtering a whole list; the same lookup as `toolkit get <category> <name>`. When rows in different groups share" +
	" the name (an override's across tenants, an endpoint's across namespaces) the call is refused with each candidate as" +
	" <group>/<name> (a service tenancy's across realms, as <realm>/<name>; model artifacts that share their model too, one per GPU config, as <model>/<name>@<config> like @2xH100); pass one of those as `name`. No row of that name is an error. Accepts env_type/env_region/env_realm overrides."

// itemResult is get_item's result: the one row, and the load's warnings.
type itemResult struct {
	Item     any      `json:"item"`
	Warnings []string `json:"warnings,omitempty"`
}

func (s *Server) handleGetItem(ctx context.Context, req *sdk.CallToolRequest, in getItemInput) (*sdk.CallToolResult, itemResult, error) {
	cat, err := domain.ParseCategory(in.Category)
	if err != nil {
		return failTool[itemResult]("get_item", fmt.Errorf("category: %w", err))
	}
	if in.Name == "" {
		return failTool[itemResult]("get_item", fmt.Errorf("name is required"))
	}
	item, warnings, err := s.pickItem(ctx, req, cat, in.Name, in.envOverride)
	if _, ok := errors.AsType[*collections.NoSuchNameError](err); ok {
		err = fmt.Errorf("%w (the name must match exactly; list tools' filter matches loosely)", err)
	}
	if err != nil {
		return failTool[itemResult]("get_item", err)
	}
	return &sdk.CallToolResult{}, itemResult{Item: item, Warnings: warnings}, nil
}

// pickItem finds cat's row named name: in its groups for a grouped
// category (so <group>/<name> resolves a shared name, as in the CLI),
// else among its list tool's rows.
func (s *Server) pickItem(ctx context.Context, req *sdk.CallToolRequest, cat domain.Category, name string, env envOverride) (any, []string, error) {
	e := s.envFor(env)
	what := cat.String()
	switch cat { //nolint:exhaustive // flat categories go through listAll
	case domain.ImportedModel:
		grouped, warnings, err := s.importedModels(ctx, e)
		return pickFromGroups(grouped, warnings, err, name, what)
	case domain.GPUNode:
		grouped, err := s.gpuNodes(ctx, e)
		return pickFromGroups(grouped, nil, err, name, what)
	case domain.GPUWorkload:
		grouped, warnings, err := s.gpuWorkloads(ctx, e)
		return pickFromGroups(grouped, warnings, err, name, what)
	case domain.DedicatedAICluster:
		grouped, warnings, err := s.dacs(ctx, e)
		return pickFromGroups(grouped, warnings, err, name, what)
	case domain.InferenceService:
		grouped, warnings, err := s.inferenceServices(ctx, e)
		return pickFromGroups(grouped, warnings, err, name, what)
	case domain.ModelArtifact:
		grouped, err := s.modelArtifacts(ctx, e)
		return pickFromGroups(grouped, nil, err, name, what)
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		return s.pickTenancyOverride(ctx, e, cat, name)
	}
	items, warnings, err := s.listAll(ctx, req, cat, env)
	if err != nil {
		return nil, nil, err
	}
	it, err := collections.PickByName(items, name, what)
	if err != nil {
		return nil, nil, err
	}
	return it, warnings, nil
}

// pickTenancyOverride is pickItem for the three tenancy overrides, which
// load together.
func (s *Server) pickTenancyOverride(ctx context.Context, env models.Environment, cat domain.Category, name string) (any, []string, error) {
	grp, err := s.tenancyGroup(ctx, env)
	what := cat.String()
	switch cat {
	case domain.LimitTenancyOverride:
		return pickFromGroups(grp.LimitTenancyOverrideMap, nil, err, name, what)
	case domain.ConsolePropertyTenancyOverride:
		return pickFromGroups(grp.ConsolePropertyTenancyOverrideMap, nil, err, name, what)
	default:
		return pickFromGroups(grp.PropertyTenancyOverrideMap, nil, err, name, what)
	}
}

// pickFromGroups finishes a grouped load for pickItem: the one row named
// name (or <group>/<name>), or the load's error.
func pickFromGroups[T any](grouped map[string][]T, warnings []string, err error, name, what string) (any, []string, error) {
	if err != nil {
		return nil, nil, err
	}
	_, it, err := collections.PickByNameGrouped(grouped, name, what)
	if err != nil {
		return nil, nil, err
	}
	return it, warnings, nil
}

// listAll is every row of a flat category, as its list_* tool returns it
// unfiltered, so a row looks the same either way.
//
//nolint:cyclop // one case per flat category, each a list tool
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
	case domain.GPUPool:
		return anyItems(s.handleListGPUPools(ctx, req, fi))
	case domain.Environment:
		return anyItems(s.handleListEnvironments(ctx, req, li))
	case domain.ServiceTenancy:
		return anyItems(s.handleListServiceTenancies(ctx, req, li))
	case domain.LimitDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("limit")))
	case domain.ConsolePropertyDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("console_property")))
	case domain.PropertyDefinition:
		return anyItems(s.handleListDefinitions(ctx, req, kind("property")))
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
