package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Alias
func aliasToRow(c domain.Category) table.Row {
	return table.Row{
		c.String(),
		strings.Join(c.GetAliases(), ", "),
	}
}

// Tenant
func tenantToRow(t models.Tenant) table.Row {
	return table.Row{
		t.Name,
		t.GetTenantID(),
		fmt.Sprint(t.IsInternal),
		t.Note,
	}
}

// LimitDefinition
func limitDefinitionToRow(val models.LimitDefinition) table.Row {
	return table.Row{
		val.Name,
		val.Description,
		val.Scope,
		val.DefaultMin,
		val.DefaultMax,
	}
}

// ConsolePropertyDefinition & PropertyDefinition
func definitionToRow[T models.Definition](val T) table.Row {
	return table.Row{
		val.GetName(),
		val.GetDescription(),
		val.GetValue(),
	}
}

// Environment
func environmentToRow(e models.Environment) table.Row {
	return table.Row{
		e.GetName(),
		e.Realm,
		e.Type,
		e.Region,
	}
}

// ServiceTenancy
func serviceTenancyToRow(s models.ServiceTenancy) table.Row {
	return table.Row{
		s.Name,
		s.Realm,
		s.Environment,
		s.HomeRegion,
		strings.Join(s.Regions, ", "),
	}
}

// GpuPool
func gpuPoolToRow(val models.GpuPool) table.Row {
	return table.Row{
		val.Name,
		val.Shape,
		fmt.Sprint(val.Size),
		fmt.Sprint(val.GetGPUs()),
		fmt.Sprint(val.IsOkeManaged),
		val.CapacityType,
	}
}

// LimitTenancyOverride
func limitTenancyOverrideToRow(val models.LimitTenancyOverride, tenant string) table.Row {
	return table.Row{
		val.Name,
		tenant,
		strings.Join(val.Regions, ", "),
		fmt.Sprint(val.Values[0].Min),
		fmt.Sprint(val.Values[0].Max),
	}
}

// ConsolePropertyTenancyOverride & PropertyTenancyOverride
func propertyTenancyOverrideToRow[T models.DefinitionOverride](val T, tenant string) table.Row {
	return table.Row{
		val.GetName(),
		tenant,
		strings.Join(val.GetRegions(), ", "),
		val.GetValue(),
	}
}

// LimitRegionalOverride
func limitRegionalOverrideToRow(val models.LimitRegionalOverride) table.Row {
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
}

// ConsolePropertyRegionalOverride & PropertyRegionalOverride
func propertyRegionalOverrideToRow[T models.DefinitionOverride](val T) table.Row {
	return table.Row{
		val.GetName(),
		strings.Join(val.GetRegions(), ", "),
		val.GetValue(),
	}
}

// BaseModel
func baseModelToRow(val models.BaseModel) table.Row {
	shape := val.GetDefaultDacShape()
	var shapeDisplay string
	if shape != nil {
		shapeDisplay = fmt.Sprintf("%dx %s", shape.QuotaUnit, shape.Name)
	}
	return table.Row{
		val.Name,
		val.DisplayName,
		val.Version,
		shapeDisplay,
		val.ParameterSize,
		fmt.Sprint(val.MaxTokens),
		val.GetFlags(),
		val.Status,
	}
}

// ModelArtifact
func modelArtifactToRow(val models.ModelArtifact, _ string) table.Row {
	return table.Row{
		val.Name,
		val.ModelName,
		val.GetGpuConfig(),
		val.TensorRTVersion,
	}
}

// GpuNode
func gpuNodeToRow(val models.GpuNode, _ string) table.Row {
	return table.Row{
		val.Name,
		val.NodePool,
		val.InstanceType,
		fmt.Sprint(val.Allocatable),
		fmt.Sprint(val.Allocatable - val.Allocated),
		fmt.Sprint(val.IsHealthy()),
		fmt.Sprint(val.IsReady),
		val.Age,
		val.GetStatus(),
	}
}

// DedicatedAICluster
func dedicatedAIClusterToRow(val models.DedicatedAICluster, tenant string) table.Row {
	unitShapeOrProfile := val.UnitShape
	if unitShapeOrProfile == "" {
		unitShapeOrProfile = val.Profile
	}
	return table.Row{
		val.Name,
		tenant,
		val.GetOwnerState(),
		val.GetUsage(),
		val.Type,
		unitShapeOrProfile,
		fmt.Sprint(val.Size),
		val.Age,
		val.Status,
	}
}
