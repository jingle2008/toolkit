package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func faultyPred[T models.Faulty](t T) bool {
	return t.IsFaulty()
}

var categoryHandlers = map[domain.Category]func(logging.Logger, *models.Dataset, *domain.ToolkitContext, string, bool) []table.Row{
	domain.Alias: func(_ logging.Logger, _ *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(domain.Categories, filter, faultyOnly, aliasToRow)
	},
	domain.Tenant: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.Tenants, filter, faultyOnly, tenantToRow)
	},
	domain.LimitDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.LimitDefinitionGroup.Values, filter, faultyOnly, limitDefinitionToRow)
	},
	domain.ConsolePropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.ConsolePropertyDefinitionGroup.Values, filter, faultyOnly, definitionToRow)
	},
	domain.PropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.PropertyDefinitionGroup.Values, filter, faultyOnly, definitionToRow)
	},
	domain.LimitTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.LimitTenancyOverrideMap, domain.Tenant, context, filter, faultyOnly, limitTenancyOverrideToRow)
	},
	domain.ConsolePropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.ConsolePropertyTenancyOverrideMap, domain.Tenant, context, filter, faultyOnly, propertyTenancyOverrideToRow)
	},
	domain.PropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.PropertyTenancyOverrideMap, domain.Tenant, context, filter, faultyOnly, propertyTenancyOverrideToRow)
	},
	domain.LimitRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.LimitRegionalOverrides, filter, faultyOnly, limitRegionalOverrideToRow)
	},
	domain.ConsolePropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.ConsolePropertyRegionalOverrides, filter, faultyOnly, propertyRegionalOverrideToRow)
	},
	domain.PropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.PropertyRegionalOverrides, filter, faultyOnly, propertyRegionalOverrideToRow)
	},
	domain.BaseModel: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.BaseModels, filter, faultyOnly, baseModelToRow)
	},
	domain.ModelArtifact: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.ModelArtifactMap, domain.BaseModel, context, filter, faultyOnly, modelArtifactToRow)
	},
	domain.Environment: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.Environments, filter, faultyOnly, environmentToRow)
	},
	domain.ServiceTenancy: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.ServiceTenancies, filter, faultyOnly, serviceTenancyToRow)
	},
	domain.GpuPool: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRows(dataset.GpuPools, filter, faultyOnly, gpuPoolToRow)
	},
	domain.GpuNode: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.GpuNodeMap, domain.GpuPool, context, filter, faultyOnly, gpuNodeToRow)
	},
	domain.DedicatedAICluster: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		return filterRowsScoped(dataset.DedicatedAIClusterMap, domain.Tenant, context, filter, faultyOnly, dedicatedAIClusterToRow)
	},
}

/*
getHeaders returns the header definitions for a given category.
If no headers are defined for the category, it returns nil.
*/
func getHeaders(category domain.Category) []header {
	if headers, exists := headerDefinitions[category]; exists {
		return headers
	}
	return nil
}

/*
getTableRows returns the table rows for a given category, using the appropriate handler.
If the context is not valid for the category, it is set to nil.
*/
func getTableRows(logger logging.Logger, dataset *models.Dataset, category domain.Category, context *domain.ToolkitContext, filter string, sortColumn string, sortAsc bool, faultyOnly bool) []table.Row {
	if context != nil && !context.Category.IsScopeOf(category) {
		context = nil
	}

	if handler, exists := categoryHandlers[category]; exists {
		rows := handler(logger, dataset, context, filter, faultyOnly)
		if sortColumn != "" && len(rows) > 0 {
			headers := getHeaders(category)
			sortRows(rows, headers, sortColumn, sortAsc)
		}
		return rows
	}

	return nil
}

/*
filterRows filters a slice of items using the provided filter and row function.
It returns a slice of table.Row for items that match the filter.
*/
func filterRows[T models.NamedFilterable](items []T, filter string, faultyOnly bool, rowFn func(T) table.Row) []table.Row {
	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
	}

	matches := collections.FilterSlice(items, nil, filter, pred)
	results := make([]table.Row, 0, len(matches))
	for _, m := range matches {
		results = append(results, rowFn(m))
	}
	return results
}

// filterRowsScoped is used for tenancy and other scoped overrides.
// Accepts a Logger interface for decoupling from zap.
func filterRowsScoped[T models.NamedFilterable](
	g map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
	faultyOnly bool,
	rowFn func(T, string) table.Row,
) []table.Row {
	var (
		key  *string
		name *string
	)

	if ctx != nil {
		if ctx.Category == scopeCategory {
			key = &ctx.Name
		} else {
			name = &ctx.Name
		}
	}

	var pred func(T) bool
	if faultyOnly {
		pred = faultyPred
	}
	matches := collections.FilterMap(g, key, name, filter, pred)
	results := make([]table.Row, 0, len(matches))
	for key, m := range matches {
		for _, v := range m {
			results = append(results, rowFn(v, key))
		}
	}
	return results
}

/*
getItemKey returns the ItemKey for a given category and table row.
*/
func getItemKey(category domain.Category, row table.Row) models.ItemKey {
	if len(row) == 0 {
		return nil
	}
	switch category {
	case domain.Tenant, domain.LimitDefinition, domain.Environment, domain.ServiceTenancy,
		domain.ConsolePropertyDefinition, domain.PropertyDefinition, domain.GpuPool,
		domain.LimitRegionalOverride, domain.ConsolePropertyRegionalOverride,
		domain.PropertyRegionalOverride, domain.ModelArtifact, domain.Alias, domain.BaseModel:
		return row[0]
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GpuNode, domain.DedicatedAICluster:
		return models.ScopedItemKey{Scope: row[1], Name: row[0]}
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

/*
findItem returns the item from the dataset for a given category and key.
*/
//nolint:cyclop
func findItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) any {
	if key == nil {
		return nil
	}
	switch category {
	case domain.Tenant:
		return findTenant(dataset, key)
	case domain.LimitDefinition:
		return findLimitDefinition(dataset, key)
	case domain.ConsolePropertyDefinition:
		return findConsolePropertyDefinition(dataset, key)
	case domain.PropertyDefinition:
		return findPropertyDefinition(dataset, key)
	case domain.LimitTenancyOverride:
		return findLimitTenancyOverride(dataset, key)
	case domain.ConsolePropertyTenancyOverride:
		return findConsolePropertyTenancyOverride(dataset, key)
	case domain.PropertyTenancyOverride:
		return findPropertyTenancyOverride(dataset, key)
	case domain.LimitRegionalOverride:
		return findLimitRegionalOverride(dataset, key)
	case domain.ConsolePropertyRegionalOverride:
		return findConsolePropertyRegionalOverride(dataset, key)
	case domain.PropertyRegionalOverride:
		return findPropertyRegionalOverride(dataset, key)
	case domain.BaseModel:
		return findBaseModel(dataset, key)
	case domain.ModelArtifact:
		return findModelArtifact(dataset, key)
	case domain.Environment:
		return findEnvironment(dataset, key)
	case domain.ServiceTenancy:
		return findServiceTenancy(dataset, key)
	case domain.GpuPool:
		return findGpuPool(dataset, key)
	case domain.GpuNode:
		return findGpuNode(dataset, key)
	case domain.DedicatedAICluster:
		return findDedicatedAICluster(dataset, key)
	case domain.Alias:
		return key
	case domain.CategoryUnknown:
		// exhaustive
	}
	return nil
}

func findTenant(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.Tenants, key.(string))
}

func findLimitDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.LimitDefinitionGroup.Values, key.(string))
}

func findConsolePropertyDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ConsolePropertyDefinitionGroup.Values, key.(string))
}

func findPropertyDefinition(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.PropertyDefinitionGroup.Values, key.(string))
}

func findLimitTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.LimitTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findConsolePropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.ConsolePropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findPropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.PropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findLimitRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.LimitRegionalOverrides, key.(string))
}

func findConsolePropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ConsolePropertyRegionalOverrides, key.(string))
}

func findPropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.PropertyRegionalOverrides, key.(string))
}

func findBaseModel(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.BaseModels, key.(string))
}

func findModelArtifact(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(string)
	for _, value := range dataset.ModelArtifactMap {
		if item := collections.FindByName(value, k); item != nil {
			return item
		}
	}
	return nil
}

func findEnvironment(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.Environments, key.(string))
}

func findServiceTenancy(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.ServiceTenancies, key.(string))
}

func findGpuPool(dataset *models.Dataset, key models.ItemKey) any {
	return collections.FindByName(dataset.GpuPools, key.(string))
}

func findGpuNode(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.GpuNodeMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

func findDedicatedAICluster(dataset *models.Dataset, key models.ItemKey) any {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.DedicatedAIClusterMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}

/*
getItemKeyString returns a string representation of the ItemKey for a given category.
*/
func getItemKeyString(category domain.Category, key models.ItemKey) string {
	switch category {
	case domain.Tenant, domain.LimitDefinition, domain.ConsolePropertyDefinition, domain.PropertyDefinition,
		domain.ConsolePropertyRegionalOverride, domain.PropertyRegionalOverride, domain.Environment,
		domain.ServiceTenancy, domain.GpuPool, domain.ModelArtifact, domain.LimitRegionalOverride,
		domain.BaseModel, domain.Alias:
		return key.(string)
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.DedicatedAICluster, domain.GpuNode:
		k := key.(models.ScopedItemKey)
		return fmt.Sprintf("%s/%s", k.Scope, k.Name)
	case domain.CategoryUnknown:
		// exhaustive
	}

	return "UNKNOWN"
}
