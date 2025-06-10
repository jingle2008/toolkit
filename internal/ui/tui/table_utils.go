package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

var categoryHandlers = map[domain.Category]func(logging.Logger, *models.Dataset, *domain.ToolkitContext, string) []table.Row{
	domain.Tenant: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return filterRows(dataset.Tenants, filter, func(t models.Tenant) table.Row {
			return TenantRow(t).ToRow("")
		})
	},
	domain.LimitDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getLimitDefinitions(dataset.LimitDefinitionGroup, filter)
	},
	domain.ConsolePropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getPropertyDefinitions(dataset.ConsolePropertyDefinitionGroup.Values, filter)
	},
	domain.PropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getPropertyDefinitions(dataset.PropertyDefinitionGroup.Values, filter)
	},
	domain.LimitTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string) []table.Row {
		return GetScopedItems(logger, dataset.LimitTenancyOverrideMap, domain.Tenant, context, filter)
	},
	domain.ConsolePropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string) []table.Row {
		return GetScopedItems(logger, dataset.ConsolePropertyTenancyOverrideMap, domain.Tenant, context, filter)
	},
	domain.PropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string) []table.Row {
		return GetScopedItems(logger, dataset.PropertyTenancyOverrideMap, domain.Tenant, context, filter)
	},
	domain.ConsolePropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getRegionalOverrides(dataset.ConsolePropertyRegionalOverrides, filter)
	},
	domain.PropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getRegionalOverrides(dataset.PropertyRegionalOverrides, filter)
	},
	domain.BaseModel: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getBaseModels(dataset.BaseModelMap, filter)
	},
	domain.ModelArtifact: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getModelArtifacts(dataset.ModelArtifacts, filter)
	},
	domain.Environment: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return filterRows(dataset.Environments, filter, func(e models.Environment) table.Row {
			return EnvironmentRow(e).ToRow("")
		})
	},
	domain.ServiceTenancy: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return filterRows(dataset.ServiceTenancies, filter, func(s models.ServiceTenancy) table.Row {
			return ServiceTenancyRow(s).ToRow("")
		})
	},
	domain.GpuPool: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string) []table.Row {
		return getGpuPools(dataset.GpuPools, filter)
	},
	domain.GpuNode: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string) []table.Row {
		return GetScopedItems(logger, dataset.GpuNodeMap, domain.GpuPool, context, filter)
	},
	domain.DedicatedAICluster: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string) []table.Row {
		return GetScopedItems(logger, dataset.DedicatedAIClusterMap, domain.Tenant, context, filter)
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
func getTableRows(logger logging.Logger, dataset *models.Dataset, category domain.Category, context *domain.ToolkitContext, filter string) []table.Row {
	if context != nil && !context.Category.IsScopeOf(category) {
		context = nil
	}

	if handler, exists := categoryHandlers[category]; exists {
		return handler(logger, dataset, context, filter)
	}

	return nil
}

/*
filterRows filters a slice of items using the provided filter and row function.
It returns a slice of table.Row for items that match the filter.
*/
func filterRows[T models.NamedFilterable](items []T, filter string, rowFn func(T) table.Row) []table.Row {
	results := make([]table.Row, 0, len(items))
	collections.FilterSlice(items, nil, filter, func(_ int, val T) bool {
		results = append(results, rowFn(val))
		return true
	})
	return results
}

/*
getGpuPools returns table rows for a slice of GpuPool, filtered by the provided filter string.
*/
func getGpuPools(pools []models.GpuPool, filter string) []table.Row {
	return filterRows(pools, filter, func(val models.GpuPool) table.Row {
		return table.Row{
			val.Name,
			val.Shape,
			fmt.Sprint(val.Size),
			fmt.Sprint(val.GetGPUs()),
			fmt.Sprint(val.IsOkeManaged),
			val.CapacityType,
		}
	})
}

/*
getLimitDefinitions returns table rows for a LimitDefinitionGroup, filtered by the provided filter string.
*/
func getLimitDefinitions(g models.LimitDefinitionGroup, filter string) []table.Row {
	return filterRows(g.Values, filter, func(val models.LimitDefinition) table.Row {
		return table.Row{
			val.Name,
			val.Description,
			val.Scope,
			val.DefaultMin,
			val.DefaultMax,
		}
	})
}

/*
getPropertyDefinitions returns table rows for a slice of Definition, filtered by the provided filter string.
*/
func getPropertyDefinitions[T models.Definition](definitions []T, filter string) []table.Row {
	return filterRows(definitions, filter, func(val T) table.Row {
		return table.Row{
			val.GetName(),
			val.GetDescription(),
			val.GetValue(),
		}
	})
}

/*
getRegionalOverrides returns table rows for a slice of DefinitionOverride, filtered by the provided filter string.
*/
func getRegionalOverrides[T models.DefinitionOverride](overrides []T, filter string) []table.Row {
	return filterRows(overrides, filter, func(val T) table.Row {
		return table.Row{
			val.GetName(),
			strings.Join(val.GetRegions(), ", "),
			val.GetValue(),
		}
	})
}

/*
getBaseModels returns table rows for a map of BaseModel, filtered by the provided filter string.
*/
func getBaseModels(m map[string]*models.BaseModel, filter string) []table.Row {
	baseModels := make([]*models.BaseModel, 0, len(m))
	for _, model := range m {
		if collections.IsMatch(model, filter, true) {
			baseModels = append(baseModels, model)
		}
	}
	sort.Slice(baseModels, func(i, j int) bool {
		return baseModels[i].GetKey() < baseModels[j].GetKey()
	})
	results := make([]table.Row, 0, len(baseModels))
	for _, val := range baseModels {
		shape := val.GetDefaultDacShape()
		var shapeDisplay string
		if shape != nil {
			shapeDisplay = fmt.Sprintf("%dx %s", shape.QuotaUnit, shape.Name)
		}
		results = append(results, table.Row{
			val.Name,
			val.Version,
			val.Type,
			shapeDisplay,
			strings.Join(val.GetCapabilities(), ", "),
			val.Category,
			fmt.Sprint(val.MaxTokens),
			val.GetFlags(),
		})
	}
	return results
}

/*
getModelArtifacts returns table rows for a slice of ModelArtifact, filtered by the provided filter string.
*/
func getModelArtifacts(artifacts []models.ModelArtifact, filter string) []table.Row {
	return filterRows(artifacts, filter, func(val models.ModelArtifact) table.Row {
		return table.Row{
			val.ModelName,
			val.GetGpuConfig(),
			val.Name,
			val.TensorRTVersion,
		}
	})
}

/*
getItemKey returns the ItemKey for a given category and table row.
*/
func getItemKey(category domain.Category, row table.Row) models.ItemKey {
	switch category {
	case domain.Tenant, domain.LimitDefinition, domain.Environment, domain.ServiceTenancy,
		domain.ConsolePropertyDefinition, domain.PropertyDefinition, domain.GpuPool,
		domain.ConsolePropertyRegionalOverride, domain.PropertyRegionalOverride:
		return row[0]
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GpuNode, domain.DedicatedAICluster:
		return models.ScopedItemKey{Scope: row[0], Name: row[1]}
	case domain.BaseModel:
		return models.BaseModelKey{Name: row[0], Version: row[1], Type: row[2]}
	case domain.ModelArtifact:
		return row[2]
	}

	return nil
}

/*
findItem returns the item from the dataset for a given category and key.
*/
//nolint:cyclop
func findItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) interface{} {
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
	default:
		return nil
	}
}

func findTenant(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.Tenants, key.(string))
}
func findLimitDefinition(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.LimitDefinitionGroup.Values, key.(string))
}
func findConsolePropertyDefinition(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.ConsolePropertyDefinitionGroup.Values, key.(string))
}
func findPropertyDefinition(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.PropertyDefinitionGroup.Values, key.(string))
}
func findLimitTenancyOverride(dataset *models.Dataset, key models.ItemKey) interface{} {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.LimitTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}
func findConsolePropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) interface{} {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.ConsolePropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}
func findPropertyTenancyOverride(dataset *models.Dataset, key models.ItemKey) interface{} {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.PropertyTenancyOverrideMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}
func findConsolePropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.ConsolePropertyRegionalOverrides, key.(string))
}
func findPropertyRegionalOverride(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.PropertyRegionalOverrides, key.(string))
}
func findBaseModel(dataset *models.Dataset, key models.ItemKey) interface{} {
	k := key.(models.BaseModelKey)
	for _, value := range dataset.BaseModelMap {
		if value.Name == k.Name &&
			value.Version == k.Version &&
			value.Type == k.Type {
			return value
		}
	}
	return nil
}
func findModelArtifact(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.ModelArtifacts, key.(string))
}
func findEnvironment(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.Environments, key.(string))
}
func findServiceTenancy(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.ServiceTenancies, key.(string))
}
func findGpuPool(dataset *models.Dataset, key models.ItemKey) interface{} {
	return collections.FindByName(dataset.GpuPools, key.(string))
}
func findGpuNode(dataset *models.Dataset, key models.ItemKey) interface{} {
	k := key.(models.ScopedItemKey)
	if items, ok := dataset.GpuNodeMap[k.Scope]; ok {
		return collections.FindByName(items, k.Name)
	}
	return nil
}
func findDedicatedAICluster(dataset *models.Dataset, key models.ItemKey) interface{} {
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
		domain.ServiceTenancy, domain.GpuPool, domain.ModelArtifact:
		return key.(string)
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.DedicatedAICluster, domain.GpuNode:
		k := key.(models.ScopedItemKey)
		return fmt.Sprintf("%s/%s", k.Scope, k.Name)
	case domain.BaseModel:
		k := key.(models.BaseModelKey)
		return fmt.Sprintf("%s-%s-%s", k.Name, k.Version, k.Type)
	}

	return "UNKNOWN"
}
