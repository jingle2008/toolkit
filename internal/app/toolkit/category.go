package toolkit

import "fmt"

type Category int

const CATEGORIES = 16

const (
	Tenant Category = iota
	LimitDefinition
	ConsolePropertyDefinition
	PropertyDefinition
	LimitTenancyOverride
	ConsolePropertyTenancyOverride
	PropertyTenancyOverride
	ConsolePropertyRegionalOverride
	PropertyRegionalOverride
	BaseModel
	ModelArtifact
	Environment
	ServiceTenancy
	GpuPool
	GpuNode
	DedicatedAICluster
)

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

func (e Category) IsScope() bool {
	switch e {
	case Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GpuPool:
		return true
	}

	return false
}

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
