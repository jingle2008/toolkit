// Package domain defines core business types and category enums for the toolkit application.
package domain

import "fmt"

// Category represents a logical grouping for toolkit data.
type Category int

const (
	// NumCategories is the total number of defined categories.
	NumCategories = 16

	// Tenant is a category for tenant-level data.
	Tenant Category = iota
	// LimitDefinition is a category for limit definitions.
	LimitDefinition
	// ConsolePropertyDefinition is a category for console property definitions.
	ConsolePropertyDefinition
	// PropertyDefinition is a category for property definitions.
	PropertyDefinition
	// LimitTenancyOverride is a category for limit tenancy overrides.
	LimitTenancyOverride
	// ConsolePropertyTenancyOverride is a category for console property tenancy overrides.
	ConsolePropertyTenancyOverride
	// PropertyTenancyOverride is a category for property tenancy overrides.
	PropertyTenancyOverride
	// ConsolePropertyRegionalOverride is a category for console property regional overrides.
	ConsolePropertyRegionalOverride
	// PropertyRegionalOverride is a category for property regional overrides.
	PropertyRegionalOverride
	// BaseModel is a category for base models.
	BaseModel
	// ModelArtifact is a category for model artifacts.
	ModelArtifact
	// Environment is a category for environments.
	Environment
	// ServiceTenancy is a category for service tenancies.
	ServiceTenancy
	// GpuPool is a category for GPU pools.
	GpuPool
	// GpuNode is a category for GPU nodes.
	GpuNode
	// DedicatedAICluster is a category for dedicated AI clusters.
	DedicatedAICluster
)

// String returns the string representation of the Category.
func (e Category) String() string {
	switch e {
	case Tenant:
		return "Tenant"
	case LimitDefinition:
		return "Limit Definition"
	case ConsolePropertyDefinition:
		return "Console Property Definition"
	case PropertyDefinition:
		return "Property Definition"
	case LimitTenancyOverride:
		return "Limit Tenancy Override"
	case ConsolePropertyTenancyOverride:
		return "Console Property Tenancy Override"
	case PropertyTenancyOverride:
		return "Property Tenancy Override"
	case ConsolePropertyRegionalOverride:
		return "Console Property Regional Override"
	case PropertyRegionalOverride:
		return "Property Regional Override"
	case BaseModel:
		return "Base Model"
	case ModelArtifact:
		return "Model Artifact"
	case Environment:
		return "Environment"
	case ServiceTenancy:
		return "Service Tenancy"
	case GpuPool:
		return "GPU Pool"
	case GpuNode:
		return "GPU Node"
	case DedicatedAICluster:
		return "Dedicated AI Cluster"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// IsScopeOf returns true if the receiver is a scope of the given category.
func (e Category) IsScopeOf(o Category) bool {
	if !e.IsScope() {
		return false
	}

	categories := e.ScopedCategories()
	for _, c := range categories {
		if c == o {
			return true
		}
	}

	return false
}

// IsScope returns true if the category is a scope category.
func (e Category) IsScope() bool {
	switch e {
	case Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GpuPool:
		return true
	}
	return false
}

// ScopedCategories returns the categories that are scoped by the receiver.
func (e Category) ScopedCategories() []Category {
	switch e {
	case Tenant:
		return []Category{
			LimitTenancyOverride,
			ConsolePropertyTenancyOverride,
			PropertyTenancyOverride,
			DedicatedAICluster,
		}
	case LimitDefinition:
		return []Category{LimitTenancyOverride}
	case ConsolePropertyDefinition:
		return []Category{
			ConsolePropertyTenancyOverride,
			ConsolePropertyRegionalOverride,
		}
	case PropertyDefinition:
		return []Category{
			PropertyTenancyOverride,
			PropertyRegionalOverride,
		}
	case GpuPool:
		return []Category{GpuNode}
	default:
		panic(fmt.Sprintf("No scoped categories for category: %s", e))
	}
}

// Definition returns the definition category for the receiver.
func (e Category) Definition() Category {
	switch e {
	case LimitTenancyOverride:
		return LimitDefinition
	case ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride:
		return ConsolePropertyDefinition
	case PropertyTenancyOverride, PropertyRegionalOverride:
		return PropertyDefinition
	case GpuNode:
		return GpuPool
	default:
		panic(fmt.Sprintf("No definition for category: %s", e))
	}
}
