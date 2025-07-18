package tui

import (
	"fmt"
	"strings"

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
	domain.Alias: func(_ logging.Logger, _ *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return filterRows(domain.Categories, filter, nil, func(c domain.Category) table.Row {
			return CategoryRow(c).ToRow("")
		})
	},
	domain.Tenant: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		var pred func(models.Tenant) bool
		if faultyOnly {
			pred = faultyPred
		}
		return filterRows(dataset.Tenants, filter, pred, func(t models.Tenant) table.Row {
			return TenantRow(t).ToRow("")
		})
	},
	domain.LimitDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getLimitDefinitions(dataset.LimitDefinitionGroup, filter)
	},
	domain.ConsolePropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getPropertyDefinitions(dataset.ConsolePropertyDefinitionGroup.Values, filter)
	},
	domain.PropertyDefinition: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getPropertyDefinitions(dataset.PropertyDefinitionGroup.Values, filter)
	},
	domain.LimitTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return GetScopedItems(logger, dataset.LimitTenancyOverrideMap, domain.Tenant, context, filter, nil)
	},
	domain.ConsolePropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return GetScopedItems(logger, dataset.ConsolePropertyTenancyOverrideMap, domain.Tenant, context, filter, nil)
	},
	domain.PropertyTenancyOverride: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return GetScopedItems(logger, dataset.PropertyTenancyOverrideMap, domain.Tenant, context, filter, nil)
	},
	domain.LimitRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getLimitRegionalOverrides(dataset.LimitRegionalOverrides, filter)
	},
	domain.ConsolePropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getRegionalOverrides(dataset.ConsolePropertyRegionalOverrides, filter)
	},
	domain.PropertyRegionalOverride: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getRegionalOverrides(dataset.PropertyRegionalOverrides, filter)
	},
	domain.BaseModel: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getBaseModels(dataset.BaseModelMap, filter)
	},
	domain.ModelArtifact: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return GetScopedItems(logger, dataset.ModelArtifactMap, domain.BaseModel, context, filter, nil)
	},
	domain.Environment: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return filterRows(dataset.Environments, filter, nil, func(e models.Environment) table.Row {
			return EnvironmentRow(e).ToRow("")
		})
	},
	domain.ServiceTenancy: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return filterRows(dataset.ServiceTenancies, filter, nil, func(s models.ServiceTenancy) table.Row {
			return ServiceTenancyRow(s).ToRow("")
		})
	},
	domain.GpuPool: func(_ logging.Logger, dataset *models.Dataset, _ *domain.ToolkitContext, filter string, _ bool) []table.Row {
		return getGpuPools(dataset.GpuPools, filter)
	},
	domain.GpuNode: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		var pred func(models.GpuNode) bool
		if faultyOnly {
			pred = faultyPred
		}
		return GetScopedItems(logger, dataset.GpuNodeMap, domain.GpuPool, context, filter, pred)
	},
	domain.DedicatedAICluster: func(logger logging.Logger, dataset *models.Dataset, context *domain.ToolkitContext, filter string, faultyOnly bool) []table.Row {
		var pred func(models.DedicatedAICluster) bool
		if faultyOnly {
			pred = faultyPred
		}
		return GetScopedItems(logger, dataset.DedicatedAIClusterMap, domain.Tenant, context, filter, pred)
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
func filterRows[T models.NamedFilterable](items []T, filter string, pred func(T) bool, rowFn func(T) table.Row) []table.Row {
	matches := collections.FilterSlice(items, nil, filter, pred)
	results := make([]table.Row, 0, len(matches))
	for _, m := range matches {
		results = append(results, rowFn(m))
	}
	return results
}

/*
getGpuPools returns table rows for a slice of GpuPool, filtered by the provided filter string.
*/
func getGpuPools(pools []models.GpuPool, filter string) []table.Row {
	return filterRows(pools, filter, nil, func(val models.GpuPool) table.Row {
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
	return filterRows(g.Values, filter, nil, func(val models.LimitDefinition) table.Row {
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
	return filterRows(definitions, filter, nil, func(val T) table.Row {
		return table.Row{
			val.GetName(),
			val.GetDescription(),
			val.GetValue(),
		}
	})
}

/*
getLimitRegionalOverrides returns table rows for a slice of LimitRegionalOverride, filtered by the provided filter string.
*/
func getLimitRegionalOverrides(overrides []models.LimitRegionalOverride, filter string) []table.Row {
	return filterRows(overrides, filter, nil, func(val models.LimitRegionalOverride) table.Row {
		minStr, maxStr := "", ""
		if len(val.Values) > 0 {
			minStr = fmt.Sprint(val.Values[0].Min)
			maxStr = fmt.Sprint(val.Values[0].Max)
		}
		return table.Row{
			val.Name,
			strings.Join(val.Regions, ", "),
			minStr,
			maxStr,
		}
	})
}

/*
getRegionalOverrides returns table rows for a slice of DefinitionOverride, filtered by the provided filter string.
*/
func getRegionalOverrides[T models.DefinitionOverride](overrides []T, filter string) []table.Row {
	return filterRows(overrides, filter, nil, func(val T) table.Row {
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

	results := make([]table.Row, 0, len(baseModels))
	for _, val := range baseModels {
		shape := val.GetDefaultDacShape()
		var shapeDisplay string
		if shape != nil {
			shapeDisplay = fmt.Sprintf("%dx %s", shape.QuotaUnit, shape.Name)
		}
		results = append(results, table.Row{
			val.Name,
			val.InternalName,
			val.Version,
			shapeDisplay,
			strings.Join(val.GetCapabilities(), "/"),
			fmt.Sprint(val.MaxTokens),
			val.GetFlags(),
		})
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
		domain.PropertyRegionalOverride, domain.ModelArtifact, domain.Alias:
		return row[0]
	case domain.BaseModel:
		return row[1] // Internal Name is now at index 1
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
	k := key.(string)
	for _, value := range dataset.BaseModelMap {
		if value.InternalName == k {
			return value
		}
	}
	return nil
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
