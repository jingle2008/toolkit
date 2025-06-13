// Package domain defines core business types and category enums for the toolkit application.
//
//go:generate stringer -type=Category
package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Category represents a logical grouping for toolkit data.
type Category int

const (
	// CategoryUnknown is the zero value for Category.
	CategoryUnknown Category = iota

	// Tenant is a category for tenant-level data.
	Tenant
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

/*
NOTE: Category iteration should use the explicit range [Tenant, DedicatedAICluster].
Do not rely on a sentinel value.
*/

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

/*
Parsing and alias logic for Category.
*/

// catLookup maps lowercased/trimmed aliases to Category values.
var catLookup = map[string]Category{
	"tenant":                          Tenant,
	"t":                               Tenant,
	"limitdefinition":                 LimitDefinition,
	"ld":                              LimitDefinition,
	"consolepropertydefinition":       ConsolePropertyDefinition,
	"cpd":                             ConsolePropertyDefinition,
	"propertydefinition":              PropertyDefinition,
	"pd":                              PropertyDefinition,
	"limittenancyoverride":            LimitTenancyOverride,
	"lto":                             LimitTenancyOverride,
	"consolepropertytenancyoverride":  ConsolePropertyTenancyOverride,
	"cpto":                            ConsolePropertyTenancyOverride,
	"propertytenancyoverride":         PropertyTenancyOverride,
	"pto":                             PropertyTenancyOverride,
	"consolepropertyregionaloverride": ConsolePropertyRegionalOverride,
	"cpro":                            ConsolePropertyRegionalOverride,
	"propertyregionaloverride":        PropertyRegionalOverride,
	"pro":                             PropertyRegionalOverride,
	"basemodel":                       BaseModel,
	"bm":                              BaseModel,
	"modelartifact":                   ModelArtifact,
	"ma":                              ModelArtifact,
	"environment":                     Environment,
	"e":                               Environment,
	"servicetenancy":                  ServiceTenancy,
	"st":                              ServiceTenancy,
	"gpupool":                         GpuPool,
	"gp":                              GpuPool,
	"gpunode":                         GpuNode,
	"gn":                              GpuNode,
	"dedicatedaicluster":              DedicatedAICluster,
	"dac":                             DedicatedAICluster,
}

// Aliases returns all canonical alias strings for autocomplete, etc.
func Aliases() []string {
	keys := make([]string, 0, len(catLookup))
	for k := range catLookup {
		keys = append(keys, k)
	}

	return keys
}

// ErrUnknownCategory is returned when a string cannot be parsed into a known Category.
var ErrUnknownCategory = errors.New("unknown category")

/*
Definition returns the definition category for the receiver.
*/
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

// ParseCategory parses a string (case-insensitive, with common aliases) into a Category enum.
func ParseCategory(s string) (Category, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if c, ok := catLookup[key]; ok {
		return c, nil
	}
	return CategoryUnknown, fmt.Errorf("parse category %q: %w", s, ErrUnknownCategory)
}
