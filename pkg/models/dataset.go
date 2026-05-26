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
	BaseModels                        []BaseModel
	ImportedModelMap                  map[string][]ImportedModel
	ModelArtifactMap                  map[string][]ModelArtifact
	Environments                      []Environment
	ServiceTenancies                  []ServiceTenancy
	GPUPools                          []GPUPool
	GPUNodeMap                        map[string][]GPUNode
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

// resolveTenantOwnedMap re-keys raw (keyed by raw TenantID — label value
// or "UNKNOWN_TENANCY") by resolved Tenant.Name when a match is
// found in d.Tenants, otherwise the raw key is preserved. setOwner
// is invoked on every value pointer with the matching tenant (nil
// when unmatched), so each item carries a back-pointer to its
// owning Tenant for downstream rendering.
func resolveTenantOwnedMap[T any](d *Dataset, raw map[string][]T, setOwner func(*T, *Tenant)) map[string][]T {
	out := make(map[string][]T, len(raw))
	suffixMap := d.buildTenantIDSuffixMap()
	for k, v := range raw {
		name := k
		var tenant *Tenant
		if idx, ok := suffixMap[k]; ok {
			tenant = &d.Tenants[idx]
			name = tenant.Name
		}
		out[name] = v
		for i := range v {
			setOwner(&v[i], tenant)
		}
	}
	return out
}

// SetDedicatedAIClusterMap sets the dedicated AI cluster map using tenant suffixes.
func (d *Dataset) SetDedicatedAIClusterMap(m map[string][]DedicatedAICluster) {
	d.DedicatedAIClusterMap = resolveTenantOwnedMap(d, m,
		func(v *DedicatedAICluster, t *Tenant) { v.Owner = t })
}

// SetImportedModelMap sets the imported model map using tenant suffixes.
func (d *Dataset) SetImportedModelMap(m map[string][]ImportedModel) {
	d.ImportedModelMap = resolveTenantOwnedMap(d, m,
		func(v *ImportedModel, t *Tenant) { v.Owner = t })
}

// ResetRealmScopedFields resets all realm-scoped fields to nil.
func (d *Dataset) ResetRealmScopedFields() {
	d.LimitTenancyOverrideMap = nil
	d.ConsolePropertyTenancyOverrideMap = nil
	d.PropertyTenancyOverrideMap = nil
	d.Tenants = nil
	d.LimitRegionalOverrides = nil
	d.ConsolePropertyRegionalOverrides = nil
	d.PropertyRegionalOverrides = nil
	d.BaseModels = nil
	d.ImportedModelMap = nil
	d.GPUPools = nil
	d.GPUNodeMap = nil
	d.DedicatedAIClusterMap = nil
}
