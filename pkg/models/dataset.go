package models

import "strings"

// Dataset holds all loaded data for the toolkit.
type Dataset struct {
	LimitDefinitionGroup              LimitDefinitionGroup
	ConsolePropertyDefinitionGroup    ConsolePropertyDefinitionGroup
	PropertyDefinitionGroup           PropertyDefinitionGroup
	ConsolePropertyTenancyOverrideMap map[string][]ConsolePropertyTenancyOverride
	LimitTenancyOverrideMap           map[string][]LimitTenancyOverride
	PropertyTenancyOverrideMap        map[string][]PropertyTenancyOverride
	ConsolePropertyRegionalOverrides  []ConsolePropertyRegionalOverride
	LimitRegionalOverrides            []LimitRegionalOverride
	PropertyRegionalOverrides         []PropertyRegionalOverride
	Tenants                           []Tenant
	BaseModelMap                      map[string]*BaseModel
	ModelArtifactMap                  map[string][]ModelArtifact
	Environments                      []Environment
	ServiceTenancies                  []ServiceTenancy
	GpuPools                          []GpuPool
	GpuNodeMap                        map[string][]GpuNode
	DedicatedAIClusterMap             map[string][]DedicatedAICluster
}

// BuildTenantIDSuffixMap builds a map from tenant ID suffix to tenant name.
func (d *Dataset) BuildTenantIDSuffixMap() map[string]string {
	suffixMap := make(map[string]string)

	for _, tenant := range d.Tenants {
		for _, id := range tenant.IDs {
			parts := strings.Split(id, ".")
			suffix := parts[len(parts)-1]
			suffixMap[suffix] = tenant.Name
		}
	}

	return suffixMap
}

// SetDedicatedAIClusterMap sets the dedicated AI cluster map using tenant suffixes.
func (d *Dataset) SetDedicatedAIClusterMap(m map[string][]DedicatedAICluster) {
	dacMap := make(map[string][]DedicatedAICluster)
	suffixMap := d.BuildTenantIDSuffixMap()

	for k, v := range m {
		tenant, ok := suffixMap[k]
		if !ok {
			tenant = k
		}

		dacMap[tenant] = v
	}

	d.DedicatedAIClusterMap = dacMap
}

// ResetScopedData resets all realm-scoped fields to nil.
func (d *Dataset) ResetScopedData() {
	d.LimitTenancyOverrideMap = nil
	d.ConsolePropertyTenancyOverrideMap = nil
	d.PropertyTenancyOverrideMap = nil
	d.Tenants = nil
	d.LimitRegionalOverrides = nil
	d.ConsolePropertyRegionalOverrides = nil
	d.PropertyRegionalOverrides = nil
	d.BaseModelMap = nil
	d.GpuPools = nil
	d.GpuNodeMap = nil
	d.DedicatedAIClusterMap = nil
}
