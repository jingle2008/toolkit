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
	ServingRuntimes                   []ServingRuntime
	ImportedModelMap                  map[string][]ImportedModel
	ModelArtifactMap                  map[string][]ModelArtifact
	Environments                      []Environment
	ServiceTenancies                  []ServiceTenancy
	GPUPools                          []GPUPool
	GPUNodeMap                        map[string][]GPUNode
	GPUWorkloadMap                    map[string][]GPUWorkload
	DedicatedAIClusterMap             map[string][]DedicatedAICluster
	InferenceServiceMap               map[string][]InferenceService
}

// FindModelByName returns the BaseModel whose Name matches name, searching
// the shared BaseModels catalog first and then every tenant's imported
// models (ImportedModel embeds BaseModel). Returns nil when nothing
// matches or name is empty. Used to resolve a DedicatedAICluster's
// ModelName to the model whose capabilities drive its metrics dashboard.
func (d *Dataset) FindModelByName(name string) *BaseModel {
	if name == "" {
		return nil
	}
	for i := range d.BaseModels {
		if d.BaseModels[i].Name == name {
			return &d.BaseModels[i]
		}
	}
	for _, ims := range d.ImportedModelMap {
		for i := range ims {
			if ims[i].Name == name {
				return &ims[i].BaseModel
			}
		}
	}
	return nil
}

// FindBaseModelByName returns the BaseModel whose Name matches name from the
// shared BaseModels catalog only (imported models are excluded). Returns nil
// on empty name or no match. Used to resolve an on-demand GPU workload's
// model to the public base model whose display name scopes its metrics.
func (d *Dataset) FindBaseModelByName(name string) *BaseModel {
	if name == "" {
		return nil
	}
	for i := range d.BaseModels {
		if d.BaseModels[i].Name == name {
			return &d.BaseModels[i]
		}
	}
	return nil
}

// buildTenantIDSuffixMap builds a map from tenant ID suffix (the part
// after the last ".", which Kubernetes tenancy-id labels carry) to tenant
// index. Tenant IDs are unique, so each suffix has one tenant.
func (d *Dataset) buildTenantIDSuffixMap() map[string]int {
	suffixMap := make(map[string]int, len(d.Tenants))
	for i, tenant := range d.Tenants {
		parts := strings.Split(tenant.ID, ".")
		suffixMap[parts[len(parts)-1]] = i
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
	if d.GPUWorkloadMap != nil {
		// Workloads loaded first may take their tenant from a DAC.
		d.SetGPUWorkloadMap(d.GPUWorkloadMap)
	}
}

// SetImportedModelMap sets the imported model map using tenant suffixes.
func (d *Dataset) SetImportedModelMap(m map[string][]ImportedModel) {
	d.ImportedModelMap = resolveTenantOwnedMap(d, m,
		func(v *ImportedModel, t *Tenant) { v.Owner = t })
}

// SetGPUWorkloadMap stores the workload map (keyed by node) and resolves
// each item's owning Tenant from its tenancy-id suffix. The node key is
// preserved (workloads are grouped by node, not tenant). Allocates a fresh
// map and does not reuse the caller's input.
//
// A pod without a tenancy-id label in a DAC's namespace (a kueue
// reservation pod, say) takes the DAC's tenancy, when the DACs are
// loaded: it runs on the tenant's cluster, and without one it would show
// no tenant and be missing from the tenant's workloads.
func (d *Dataset) SetGPUWorkloadMap(m map[string][]GPUWorkload) {
	if dacTenants := d.dacTenantsByName(); len(dacTenants) > 0 {
		for _, items := range m {
			for i := range items {
				if items[i].TenantID == "" {
					items[i].TenantID = dacTenants[items[i].Namespace]
				}
			}
		}
	}
	d.GPUWorkloadMap = resolveOwnersInPlace(d, m,
		func(v *GPUWorkload) string { return v.TenantID },
		func(v *GPUWorkload, t *Tenant) { v.Owner = t })
}

// SetInferenceServiceMap stores the inference service map (keyed by
// namespace) and resolves each item's owning Tenant from its tenancy-id
// suffix. The namespace key is preserved. Allocates a fresh map and does
// not reuse the caller's input.
func (d *Dataset) SetInferenceServiceMap(m map[string][]InferenceService) {
	d.InferenceServiceMap = resolveOwnersInPlace(d, m,
		func(v *InferenceService) string { return v.TenantID },
		func(v *InferenceService, t *Tenant) { v.Owner = t })
}

// dacTenantsByName maps each loaded DAC's name (its namespace's name) to
// its raw tenancy id.
func (d *Dataset) dacTenantsByName() map[string]string {
	out := make(map[string]string)
	for _, dacs := range d.DedicatedAIClusterMap {
		for _, dac := range dacs {
			if dac.TenantID != "" {
				out[dac.Name] = dac.TenantID
			}
		}
	}
	return out
}

// resolveOwnersInPlace is the non-re-keying counterpart of
// resolveTenantOwnedMap, for maps grouped by something other than the
// tenant: each item's owner is resolved from its own tenant ID (nil when
// unmatched) and the original keys are kept.
func resolveOwnersInPlace[T any](d *Dataset, m map[string][]T, tenantID func(*T) string, setOwner func(*T, *Tenant)) map[string][]T {
	suffixMap := d.buildTenantIDSuffixMap()
	out := make(map[string][]T, len(m))
	for k, v := range m {
		for i := range v {
			var tenant *Tenant
			if idx, ok := suffixMap[tenantID(&v[i])]; ok {
				tenant = &d.Tenants[idx]
			}
			setOwner(&v[i], tenant)
		}
		out[k] = v
	}
	return out
}
