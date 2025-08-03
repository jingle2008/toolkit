// Package domain defines core business types and category enums for the toolkit application.
//
//go:generate stringer -type=Category
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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
	// LimitRegionalOverride is a category for limit regional overrides.
	LimitRegionalOverride
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
	// Alias is a category for reporting all aliases.
	Alias
)

/*
NOTE: Category iteration should use the explicit range [Tenant, Alias].
Do not rely on a sentinel value.
*/

// IsScopeOf returns true if the receiver is a scope of the given category.
func (e Category) IsScopeOf(o Category) bool {
	if !e.IsScope() {
		return false
	}

	categories := e.ScopedCategories()
	return slices.Contains(categories, o)
}

// IsScope returns true if the category is a scope category.
func (e Category) IsScope() bool {
	switch e { //nolint:exhaustive
	case Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GpuPool:
		return true
	}
	return false
}

// ScopedCategories returns the categories that are scoped by the receiver.
func (e Category) ScopedCategories() []Category {
	switch e { //nolint:exhaustive
	case Tenant:
		return []Category{
			LimitTenancyOverride,
			ConsolePropertyTenancyOverride,
			PropertyTenancyOverride,
			DedicatedAICluster,
		}
	case LimitDefinition:
		return []Category{LimitTenancyOverride, LimitRegionalOverride}
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
		return nil
	}
}

/*
GetAliases returns a list of aliases for the Category.
*/
func (e Category) GetAliases() []string {
	cat := e.String()
	short := GetInitials(cat)
	aliases := []string{strings.ToLower(short), strings.ToLower(cat)}

	if e == DedicatedAICluster {
		aliases = append(aliases, "dac")
	}
	return aliases
}

/*
GetName returns the string name of the Category.
*/
func (e Category) GetName() string {
	return e.String()
}

/*
GetFilterableFields returns the filterable fields for the Category.
*/
func (e Category) GetFilterableFields() []string {
	return e.GetAliases()
}

/*
IsFaulty returns whether the Category is considered faulty.
*/
func (e Category) IsFaulty() bool {
	return false
}

/*
GetInitials returns the initials of a string, used for aliasing.
*/
func GetInitials(s string) string {
	re := regexp.MustCompile(`[A-Z]`)
	initials := re.FindAllString(s, -1)
	return strings.Join(initials, "")
}

var (
	aliasToCat map[string]Category

	// Aliases contains all known aliases for categories.
	Aliases []string

	// Categories contains all defined categories.
	Categories []Category
)

func init() {
	aliasToCat = make(map[string]Category)

	for c := Tenant; c <= Alias; c++ {
		for _, a := range c.GetAliases() {
			aliasToCat[a] = c
		}
		Categories = append(Categories, c)
	}

	Aliases = make([]string, 0, len(aliasToCat))
	for k := range aliasToCat {
		Aliases = append(Aliases, k)
	}
}

// ErrUnknownCategory is returned when a string cannot be parsed into a known Category.
var ErrUnknownCategory = errors.New("unknown category")

/*
Definition returns the definition category for the receiver.
*/
func (e Category) Definition() Category {
	switch e { //nolint:exhaustive
	case LimitTenancyOverride, LimitRegionalOverride:
		return LimitDefinition
	case ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride:
		return ConsolePropertyDefinition
	case PropertyTenancyOverride, PropertyRegionalOverride:
		return PropertyDefinition
	case GpuNode:
		return GpuPool
	default:
		return Category(-1)
	}
}

/*
ParseCategory parses a string (case-insensitive, with common aliases) into a Category enum.
*/
func ParseCategory(s string) (Category, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if c, ok := aliasToCat[key]; ok {
		return c, nil
	}
	return CategoryUnknown, fmt.Errorf("parse category %q: %w", s, ErrUnknownCategory)
}
