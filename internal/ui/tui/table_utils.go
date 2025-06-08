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
func findItem(dataset *models.Dataset, category domain.Category, key models.ItemKey) interface{} { //nolint:cyclop
	var item interface{}

	switch category {
	case domain.Tenant:
		item = collections.FindByName(dataset.Tenants, key.(string))
	case domain.LimitDefinition:
		item = collections.FindByName(dataset.LimitDefinitionGroup.Values, key.(string))
	case domain.ConsolePropertyDefinition:
		item = collections.FindByName(dataset.ConsolePropertyDefinitionGroup.Values, key.(string))
	case domain.PropertyDefinition:
		item = collections.FindByName(dataset.PropertyDefinitionGroup.Values, key.(string))
	case domain.LimitTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.LimitTenancyOverrideMap[k.Scope]; ok {
			item = collections.FindByName(items, k.Name)
		}
	case domain.ConsolePropertyTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.ConsolePropertyTenancyOverrideMap[k.Scope]; ok {
			item = collections.FindByName(items, k.Name)
		}
	case domain.PropertyTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.PropertyTenancyOverrideMap[k.Scope]; ok {
			item = collections.FindByName(items, k.Name)
		}
	case domain.ConsolePropertyRegionalOverride:
		item = collections.FindByName(dataset.ConsolePropertyRegionalOverrides, key.(string))
	case domain.PropertyRegionalOverride:
		item = collections.FindByName(dataset.PropertyRegionalOverrides, key.(string))
	case domain.BaseModel:
		k := key.(models.BaseModelKey)
		for _, value := range dataset.BaseModelMap {
			if value.Name == k.Name &&
				value.Version == k.Version &&
				value.Type == k.Type {
				item = value
			}
		}
	case domain.ModelArtifact:
		item = collections.FindByName(dataset.ModelArtifacts, key.(string))
	case domain.Environment:
		item = collections.FindByName(dataset.Environments, key.(string))
	case domain.ServiceTenancy:
		item = collections.FindByName(dataset.ServiceTenancies, key.(string))
	case domain.GpuPool:
		item = collections.FindByName(dataset.GpuPools, key.(string))
	case domain.GpuNode:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.GpuNodeMap[k.Scope]; ok {
			item = collections.FindByName(items, k.Name)
		}
	case domain.DedicatedAICluster:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.DedicatedAIClusterMap[k.Scope]; ok {
			item = collections.FindByName(items, k.Name)
		}
	}

	return item
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
