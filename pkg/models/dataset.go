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
	GPUWorkloadMap                    map[string][]GPUWorkload
	DedicatedAIClusterMap             map[string][]DedicatedAICluster
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

// SetGPUWorkloadMap stores the workload map (keyed by node) and resolves
// each item's owning Tenant from its tenancy-id suffix. The node key is
// preserved (workloads are grouped by node, not tenant). Allocates a fresh
// map and does not reuse the caller's input.
func (d *Dataset) SetGPUWorkloadMap(m map[string][]GPUWorkload) {
	suffixMap := d.buildTenantIDSuffixMap()
	out := make(map[string][]GPUWorkload, len(m))
	for k, v := range m {
		for i := range v {
			if idx, ok := suffixMap[v[i].TenantID]; ok {
				v[i].Owner = &d.Tenants[idx]
			} else {
				v[i].Owner = nil
			}
		}
		out[k] = v
	}
	d.GPUWorkloadMap = out
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
	d.GPUWorkloadMap = nil
	d.DedicatedAIClusterMap = nil
}

// MergeReloadedRepoData copies the repo-owned fields from fresh into d while
// preserving the lazily-loaded, k8s-backed fields already present in d
// (BaseModels, ImportedModelMap, GPUPools, GPUNodeMap, GPUWorkloadMap,
// DedicatedAIClusterMap). It is used when a working-tree change triggers a
// dataset reload: LoadDataset repopulates only the repo-owned fields, so a
// wholesale assignment would wipe live k8s data. New repo-owned fields added
// to Dataset are carried across automatically; only the small, stable set of
// k8s fields is enumerated here.
func (d *Dataset) MergeReloadedRepoData(fresh *Dataset) {
	fresh.BaseModels = d.BaseModels
	fresh.ImportedModelMap = d.ImportedModelMap
	fresh.GPUPools = d.GPUPools
	fresh.GPUNodeMap = d.GPUNodeMap
	fresh.GPUWorkloadMap = d.GPUWorkloadMap
	fresh.DedicatedAIClusterMap = d.DedicatedAIClusterMap
	*d = *fresh
}
