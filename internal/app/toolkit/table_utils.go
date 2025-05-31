package toolkit

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

var headerDefinitions = map[Category][]header{
	Tenant: {
		{text: "Name", ratio: 0.25},
		{text: "OCID", ratio: 0.65},
		{text: "LO/CPO/PO", ratio: 0.1},
	},
	LimitDefinition: {
		{text: "Name", ratio: 0.32},
		{text: "Description", ratio: 0.48},
		{text: "Scope", ratio: 0.08},
		{text: "Min", ratio: 0.06},
		{text: "Max", ratio: 0.06},
	},
	ConsolePropertyDefinition: {
		{text: "Name", ratio: 0.38},
		{text: "Description", ratio: 0.5},
		{text: "Value", ratio: 0.12},
	},
	PropertyDefinition: {
		{text: "Name", ratio: 0.38},
		{text: "Description", ratio: 0.5},
		{text: "Value", ratio: 0.12},
	},
	LimitTenancyOverride: {
		{text: "Tenant", ratio: 0.24},
		{text: "Limit", ratio: 0.4},
		{text: "Regions", ratio: 0.2},
		{text: "Min", ratio: 0.08},
		{text: "Max", ratio: 0.08},
	},
	ConsolePropertyTenancyOverride: {
		{text: "Tenant", ratio: 0.25},
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.25},
		{text: "Value", ratio: 0.1},
	},
	PropertyTenancyOverride: {
		{text: "Tenant", ratio: 0.25},
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.25},
		{text: "Value", ratio: 0.1},
	},
	ConsolePropertyRegionalOverride: {
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.3},
		{text: "Value", ratio: 0.3},
	},
	PropertyRegionalOverride: {
		{text: "Property", ratio: 0.4},
		{text: "Regions", ratio: 0.3},
		{text: "Value", ratio: 0.3},
	},
	BaseModel: {
		{text: "Name", ratio: 0.26},
		{text: "Version", ratio: 0.08},
		{text: "Type", ratio: 0.08},
		{text: "DAC Shape", ratio: 0.16},
		{text: "Capabilities", ratio: 0.18},
		{text: "Category", ratio: 0.08},
		{text: "Max Tokens", ratio: 0.08},
		{text: "Flags", ratio: 0.08},
	},
	ModelArtifact: {
		{text: "Model Name", ratio: 0.3},
		{text: "GPU Config", ratio: 0.1},
		{text: "Artifact Name", ratio: 0.5},
		{text: "TRT Version", ratio: 0.1},
	},
	Environment: {
		{text: "Name", ratio: 0.2},
		{text: "Realm", ratio: 0.15},
		{text: "Type", ratio: 0.15},
		{text: "Region", ratio: 0.5},
	},
	ServiceTenancy: {
		{text: "Name", ratio: 0.15},
		{text: "Realm", ratio: 0.1},
		{text: "Environment", ratio: 0.1},
		{text: "Home Region", ratio: 0.15},
		{text: "Regions", ratio: 0.5},
	},
	GpuPool: {
		{text: "Name", ratio: 0.3},
		{text: "Shape", ratio: 0.3},
		{text: "Size", ratio: 0.1},
		{text: "GPUs", ratio: 0.1},
		{text: "OKE Managed", ratio: 0.1},
		{text: "Capacity Type", ratio: 0.1},
	},
	GpuNode: {
		{text: "PoolName", ratio: 0.2},
		{text: "Name", ratio: 0.15},
		{text: "Instance Type", ratio: 0.15},
		{text: "Total", ratio: 0.08},
		{text: "Free", ratio: 0.08},
		{text: "Healthy", ratio: 0.08},
		{text: "Ready", ratio: 0.08},
		{text: "Status", ratio: 0.18},
	},
	DedicatedAICluster: {
		{text: "Tenant", ratio: 0.2},
		{text: "Name", ratio: 0.44},
		{text: "Type", ratio: 0.07},
		{text: "Unit Shape/Profile", ratio: 0.16},
		{text: "Size", ratio: 0.05},
		{text: "Status", ratio: 0.08},
	},
}

var categoryHandlers = map[Category]func(*models.Dataset, *Context, string) []table.Row{
	Tenant: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getTenants(dataset.Tenants, filter)
	},
	LimitDefinition: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getLimitDefinitions(dataset.LimitDefinitionGroup, filter)
	},
	ConsolePropertyDefinition: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getPropertyDefinitions(dataset.ConsolePropertyDefinitionGroup.Values, filter)
	},
	PropertyDefinition: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getPropertyDefinitions(dataset.PropertyDefinitionGroup.Values, filter)
	},
	LimitTenancyOverride: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getScopedItems(dataset.LimitTenancyOverrideMap, Tenant, context, filter)
	},
	ConsolePropertyTenancyOverride: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getScopedItems(dataset.ConsolePropertyTenancyOverrideMap, Tenant, context, filter)
	},
	PropertyTenancyOverride: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getScopedItems(dataset.PropertyTenancyOverrideMap, Tenant, context, filter)
	},
	ConsolePropertyRegionalOverride: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getRegionalOverrides(dataset.ConsolePropertyRegionalOverrides, filter)
	},
	PropertyRegionalOverride: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getRegionalOverrides(dataset.PropertyRegionalOverrides, filter)
	},
	BaseModel: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getBaseModels(dataset.BaseModelMap, filter)
	},
	ModelArtifact: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getModelArtifacts(dataset.ModelArtifacts, filter)
	},
	Environment: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getEnvironments(dataset.Environments, filter)
	},
	ServiceTenancy: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getServiceTenancies(dataset.ServiceTenancies, filter)
	},
	GpuPool: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getGpuPools(dataset.GpuPools, filter)
	},
	GpuNode: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getScopedItems(dataset.GpuNodeMap, GpuPool, context, filter)
	},
	DedicatedAICluster: func(dataset *models.Dataset, context *Context, filter string) []table.Row {
		return getScopedItems(dataset.DedicatedAIClusterMap, Tenant, context, filter)
	},
}

func getHeaders(category Category) []header {
	if headers, exists := headerDefinitions[category]; exists {
		return headers
	}
	return nil
}

func getTableRows(dataset *models.Dataset, category Category, context *Context, filter string) []table.Row {
	if context != nil && !context.Category.IsScopeOf(category) {
		context = nil
	}

	if handler, exists := categoryHandlers[category]; exists {
		return handler(dataset, context, filter)
	}

	return nil
}

func getTenants(tenants []models.Tenant, filter string) []table.Row {
	results := make([]table.Row, 0, len(tenants))

	utils.FilterSlice(tenants, nil, filter, func(i int, val models.Tenant) bool {
		results = append(results, table.Row{
			val.Name,
			val.GetTenantId(),
			val.GetOverrides(),
		})
		return true
	})

	return results
}

func getEnvironments(envs []models.Environment, filter string) []table.Row {
	results := make([]table.Row, 0, len(envs))

	utils.FilterSlice(envs, nil, filter, func(i int, val models.Environment) bool {
		results = append(results, table.Row{
			val.GetName(),
			val.Realm,
			val.Type,
			val.Region,
		})
		return true
	})

	return results
}

func getGpuPools(pools []models.GpuPool, filter string) []table.Row {
	results := make([]table.Row, 0, len(pools))

	utils.FilterSlice(pools, nil, filter, func(i int, val models.GpuPool) bool {
		results = append(results, table.Row{
			val.Name,
			val.Shape,
			fmt.Sprint(val.Size),
			fmt.Sprint(val.GetGPUs()),
			fmt.Sprint(val.IsOkeManaged),
			val.CapacityType,
		})
		return true
	})

	return results
}

func getServiceTenancies(tenancies []models.ServiceTenancy, filter string) []table.Row {
	results := make([]table.Row, 0, len(tenancies))

	utils.FilterSlice(tenancies, nil, filter, func(i int, val models.ServiceTenancy) bool {
		results = append(results, table.Row{
			val.Name,
			val.Realm,
			val.Environment,
			val.HomeRegion,
			strings.Join(val.Regions, ", "),
		})
		return true
	})

	return results
}

func getLimitDefinitions(g models.LimitDefinitionGroup, filter string) []table.Row {
	results := make([]table.Row, 0, len(g.Values))

	utils.FilterSlice(g.Values, nil, filter, func(i int, val models.LimitDefinition) bool {
		results = append(results, table.Row{
			val.Name,
			val.Description,
			val.Scope,
			val.DefaultMin,
			val.DefaultMax,
		})
		return true
	})

	return results
}

func getPropertyDefinitions[T models.Definition](definitions []T, filter string) []table.Row {
	results := make([]table.Row, 0, len(definitions))

	utils.FilterSlice(definitions, nil, filter, func(i int, val T) bool {
		results = append(results, table.Row{
			val.GetName(),
			val.GetDescription(),
			val.GetValue(),
		})
		return true
	})

	return results
}

func getTableRow(tenant string, item interface{}) table.Row {
	switch val := item.(type) {
	case models.LimitTenancyOverride:
		return table.Row{
			tenant,
			val.Name,
			strings.Join(val.Regions, ", "),
			fmt.Sprint(val.Values[0].Min),
			fmt.Sprint(val.Values[0].Max),
		}

	case models.ConsolePropertyTenancyOverride:
		return table.Row{
			tenant,
			val.Name,
			strings.Join(val.GetRegions(), ", "),
			val.GetValue(),
		}

	case models.PropertyTenancyOverride:
		return table.Row{
			tenant,
			val.Name,
			strings.Join(val.GetRegions(), ", "),
			val.GetValue(),
		}

	case models.GpuNode:
		return table.Row{
			val.NodePool,
			val.Name,
			val.InstanceType,
			fmt.Sprint(val.Allocatable),
			fmt.Sprint(val.Allocatable - val.Allocated),
			fmt.Sprint(val.IsHealthy),
			fmt.Sprint(val.IsReady),
			val.GetStatus(),
		}

	case models.DedicatedAICluster:
		// Show UnitShape (v1) or Profile (v2) in the same column
		unitShapeOrProfile := val.UnitShape
		if unitShapeOrProfile == "" {
			unitShapeOrProfile = val.Profile
		}
		return table.Row{
			tenant,
			val.Name,
			val.Type,
			unitShapeOrProfile,
			fmt.Sprint(val.Size),
			val.Status,
		}

	default:
		log.Printf("value is of type: %T\n", val)
	}

	return nil
}

func getScopedItems[T models.NamedFilterable](g map[string][]T,
	scopeCategory Category, context *Context, filter string,
) []table.Row {
	var (
		key  *string
		name *string
	)

	if context != nil {
		if context.Category == scopeCategory {
			key = &context.Name
		} else {
			name = &context.Name
		}
	}

	return utils.FilterMap(g, key, name, filter,
		func(s string, v T) table.Row {
			return getTableRow(s, v)
		})
}

func getRegionalOverrides[T models.DefinitionOverride](overrides []T, filter string) []table.Row {
	results := make([]table.Row, 0, len(overrides))

	utils.FilterSlice(overrides, nil, filter, func(i int, val T) bool {
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

	results := make([]table.Row, 0, len(m))
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
	results := make([]table.Row, 0, len(artifacts))

	utils.FilterSlice(artifacts, nil, filter, func(i int, val models.ModelArtifact) bool {
		results = append(results, table.Row{
			val.ModelName,
			val.GetGpuConfig(),
			val.Name,
			val.TensorRTVersion,
		})
		return true
	})

	return results
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

	return "UNKOWN"
}
