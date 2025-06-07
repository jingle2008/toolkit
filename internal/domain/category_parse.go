// Package domain provides business types and category enums for the toolkit application.
package domain

import (
	"errors"
	"fmt"
	"strings"
)

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

// ErrUnknownCategory is returned when a string cannot be parsed into a known Category.
var ErrUnknownCategory = errors.New("unknown category")

// ParseCategory parses a string (case-insensitive, with common aliases) into a Category enum.
func ParseCategory(s string) (Category, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if c, ok := catLookup[key]; ok {
		return c, nil
	}
	return CategoryUnknown, fmt.Errorf("parse category %q: %w", s, ErrUnknownCategory)
}
