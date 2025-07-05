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

// buildTenantIDSuffixMap builds a map from tenant ID suffix to tenant index.
func (d *Dataset) buildTenantIDSuffixMap() map[string]int {
	suffixMap := make(map[string]int)

	for i, tenant := range d.Tenants {
		for _, id := range tenant.IDs {
			parts := strings.Split(id, ".")
			suffix := parts[len(parts)-1]
			suffixMap[suffix] = i
		}
	}

	return suffixMap
}

// SetDedicatedAIClusterMap sets the dedicated AI cluster map using tenant suffixes.
func (d *Dataset) SetDedicatedAIClusterMap(m map[string][]DedicatedAICluster) {
	dacMap := make(map[string][]DedicatedAICluster)
	suffixMap := d.buildTenantIDSuffixMap()

	for k, v := range m {
		name := k
		var tenant *Tenant
		if idx, ok := suffixMap[k]; ok {
			tenant = &d.Tenants[idx]
			name = tenant.Name
		}

		dacMap[name] = v
		for i := range v {
			v[i].Owner = tenant
		}
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
