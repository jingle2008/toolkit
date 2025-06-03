package toolkit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

var categoryHandlers = map[Category]func(*zap.Logger, *models.Dataset, *AppContext, string) []table.Row{
	Tenant: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getTenants(dataset.Tenants, filter)
	},
	LimitDefinition: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getLimitDefinitions(dataset.LimitDefinitionGroup, filter)
	},
	ConsolePropertyDefinition: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getPropertyDefinitions(dataset.ConsolePropertyDefinitionGroup.Values, filter)
	},
	PropertyDefinition: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getPropertyDefinitions(dataset.PropertyDefinitionGroup.Values, filter)
	},
	LimitTenancyOverride: func(logger *zap.Logger, dataset *models.Dataset, context *AppContext, filter string) []table.Row {
		return getScopedItems(logger, dataset.LimitTenancyOverrideMap, Tenant, context, filter)
	},
	ConsolePropertyTenancyOverride: func(logger *zap.Logger, dataset *models.Dataset, context *AppContext, filter string) []table.Row {
		return getScopedItems(logger, dataset.ConsolePropertyTenancyOverrideMap, Tenant, context, filter)
	},
	PropertyTenancyOverride: func(logger *zap.Logger, dataset *models.Dataset, context *AppContext, filter string) []table.Row {
		return getScopedItems(logger, dataset.PropertyTenancyOverrideMap, Tenant, context, filter)
	},
	ConsolePropertyRegionalOverride: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getRegionalOverrides(dataset.ConsolePropertyRegionalOverrides, filter)
	},
	PropertyRegionalOverride: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getRegionalOverrides(dataset.PropertyRegionalOverrides, filter)
	},
	BaseModel: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getBaseModels(dataset.BaseModelMap, filter)
	},
	ModelArtifact: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getModelArtifacts(dataset.ModelArtifacts, filter)
	},
	Environment: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getEnvironments(dataset.Environments, filter)
	},
	ServiceTenancy: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getServiceTenancies(dataset.ServiceTenancies, filter)
	},
	GpuPool: func(logger *zap.Logger, dataset *models.Dataset, _ *AppContext, filter string) []table.Row {
		return getGpuPools(dataset.GpuPools, filter)
	},
	GpuNode: func(logger *zap.Logger, dataset *models.Dataset, context *AppContext, filter string) []table.Row {
		return getScopedItems(logger, dataset.GpuNodeMap, GpuPool, context, filter)
	},
	DedicatedAICluster: func(logger *zap.Logger, dataset *models.Dataset, context *AppContext, filter string) []table.Row {
		return getScopedItems(logger, dataset.DedicatedAIClusterMap, Tenant, context, filter)
	},
}

func getHeaders(category Category) []header {
	if headers, exists := headerDefinitions[category]; exists {
		return headers
	}
	return nil
}

func getTableRows(logger *zap.Logger, dataset *models.Dataset, category Category, context *AppContext, filter string) []table.Row {
	if context != nil && !context.Category.IsScopeOf(category) {
		context = nil
	}

	if handler, exists := categoryHandlers[category]; exists {
		return handler(logger, dataset, context, filter)
	}

	return nil
}

func filterRows[T models.NamedFilterable](items []T, filter string, rowFn func(T) table.Row) []table.Row {
	results := make([]table.Row, 0, len(items))
	utils.FilterSlice(items, nil, filter, func(_ int, val T) bool {
		results = append(results, rowFn(val))
		return true
	})
	return results
}

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

func getPropertyDefinitions[T models.Definition](definitions []T, filter string) []table.Row {
	return filterRows(definitions, filter, func(val T) table.Row {
		return table.Row{
			val.GetName(),
			val.GetDescription(),
			val.GetValue(),
		}
	})
}

func getTableRow(logger *zap.Logger, tenant string, item interface{}) table.Row {
	switch val := item.(type) {
	case models.LimitTenancyOverride:
		// Use adapter function for RowMarshaler pattern
		return LimitTenancyOverrideToRow(tenant, val)

	case models.ConsolePropertyTenancyOverride:
		// Use adapter function for RowMarshaler pattern
		return ConsolePropertyTenancyOverrideToRow(tenant, val)

	case models.PropertyTenancyOverride:
		// Use adapter function for RowMarshaler pattern
		return PropertyTenancyOverrideToRow(tenant, val)

	case models.GpuNode:
		// Use adapter function for RowMarshaler pattern
		return GpuNodeToRow(tenant, val)

	case models.DedicatedAICluster:
		// Use adapter function for RowMarshaler pattern
		return DedicatedAIClusterToRow(tenant, val)

	default:
		if logger != nil {
			logger.Warn("unexpected type in getTableRow",
				zap.String("type", fmt.Sprintf("%T", val)),
			)
		}
	}

	return nil
}

func getRegionalOverrides[T models.DefinitionOverride](overrides []T, filter string) []table.Row {
	results := make([]table.Row, 0, len(overrides))

	utils.FilterSlice(overrides, nil, filter, func(_ int, val T) bool {
		results = append(results, table.Row{
			val.GetName(),
			strings.Join(val.GetRegions(), ", "),
			val.GetValue(),
		})
		return true
	})

	return results
}

func getBaseModels(m map[string]*models.BaseModel, filter string) []table.Row {
	baseModels := make([]*models.BaseModel, 0, len(m))
	for _, model := range m {
		if utils.IsMatch(model, filter, true) {
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

func getItemKey(category Category, row table.Row) models.ItemKey {
	switch category {
	case Tenant, LimitDefinition, Environment, ServiceTenancy,
		ConsolePropertyDefinition, PropertyDefinition, GpuPool,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride:
		return row[0]
	case LimitTenancyOverride, ConsolePropertyTenancyOverride,
		PropertyTenancyOverride, GpuNode, DedicatedAICluster:
		return models.ScopedItemKey{Scope: row[0], Name: row[1]}
	case BaseModel:
		return models.BaseModelKey{Name: row[0], Version: row[1], Type: row[2]}
	case ModelArtifact:
		return row[2]
	}

	return nil
}

func findItem(dataset *models.Dataset, category Category, key models.ItemKey) interface{} {
	var item interface{}

	switch category {
	case Tenant:
		item = utils.FindByName(dataset.Tenants, key.(string))
	case LimitDefinition:
		item = utils.FindByName(dataset.LimitDefinitionGroup.Values, key.(string))
	case ConsolePropertyDefinition:
		item = utils.FindByName(dataset.ConsolePropertyDefinitionGroup.Values, key.(string))
	case PropertyDefinition:
		item = utils.FindByName(dataset.PropertyDefinitionGroup.Values, key.(string))
	case LimitTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.LimitTenancyOverrideMap[k.Scope]; ok {
			item = utils.FindByName(items, k.Name)
		}
	case ConsolePropertyTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.ConsolePropertyTenancyOverrideMap[k.Scope]; ok {
			item = utils.FindByName(items, k.Name)
		}
	case PropertyTenancyOverride:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.PropertyTenancyOverrideMap[k.Scope]; ok {
			item = utils.FindByName(items, k.Name)
		}
	case ConsolePropertyRegionalOverride:
		item = utils.FindByName(dataset.ConsolePropertyRegionalOverrides, key.(string))
	case PropertyRegionalOverride:
		item = utils.FindByName(dataset.PropertyRegionalOverrides, key.(string))
	case BaseModel:
		k := key.(models.BaseModelKey)
		for _, value := range dataset.BaseModelMap {
			if value.Name == k.Name &&
				value.Version == k.Version &&
				value.Type == k.Type {
				item = value
			}
		}
	case ModelArtifact:
		item = utils.FindByName(dataset.ModelArtifacts, key.(string))
	case Environment:
		item = utils.FindByName(dataset.Environments, key.(string))
	case ServiceTenancy:
		item = utils.FindByName(dataset.ServiceTenancies, key.(string))
	case GpuPool:
		item = utils.FindByName(dataset.GpuPools, key.(string))
	case GpuNode:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.GpuNodeMap[k.Scope]; ok {
			item = utils.FindByName(items, k.Name)
		}
	case DedicatedAICluster:
		k := key.(models.ScopedItemKey)
		if items, ok := dataset.DedicatedAIClusterMap[k.Scope]; ok {
			item = utils.FindByName(items, k.Name)
		}
	}

	return item
}

func getItemKeyString(category Category, key models.ItemKey) string {
	switch category {
	case Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride, Environment,
		ServiceTenancy, GpuPool, ModelArtifact:
		return key.(string)
	case LimitTenancyOverride, ConsolePropertyTenancyOverride,
		PropertyTenancyOverride, DedicatedAICluster, GpuNode:
		k := key.(models.ScopedItemKey)
		return fmt.Sprintf("%s/%s", k.Scope, k.Name)
	case BaseModel:
		k := key.(models.BaseModelKey)
		return fmt.Sprintf("%s-%s-%s", k.Name, k.Version, k.Type)
	}

	return "UNKNOWN"
}
