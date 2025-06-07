// Package domain defines core business types and category enums for the toolkit application.
//
//go:generate stringer -type=Category
package domain

// Category represents a logical grouping for toolkit data.
type Category int

const (
	// CategorySentinelLast is always the last valid category (for iteration).
	CategorySentinelLast = DedicatedAICluster

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
		// Instead of panic, return nil to indicate no scoped categories.
		return nil
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
		// Instead of panic, return Category(-1) to indicate no definition.
		return Category(-1)
	}
}
