package tui

import "github.com/jingle2008/toolkit/pkg/models"

// resetRealmScopedFields nils what a switch to another realm invalidates:
// the realm's tenants and overrides, and everything
// resetEnvironmentScopedFields nils.
func resetRealmScopedFields(d *models.Dataset) {
	d.LimitTenancyOverrideMap = nil
	d.ConsolePropertyTenancyOverrideMap = nil
	d.PropertyTenancyOverrideMap = nil
	d.Tenants = nil
	d.LimitRegionalOverrides = nil
	d.ConsolePropertyRegionalOverrides = nil
	d.PropertyRegionalOverrides = nil
	resetEnvironmentScopedFields(d)
}

// resetEnvironmentScopedFields nils what a switch to another environment
// in the same realm invalidates: the region's GPU pools and the cluster's
// k8s-backed categories. Tenants and overrides are loaded per realm, so
// they stay.
func resetEnvironmentScopedFields(d *models.Dataset) {
	d.BaseModels = nil
	d.ServingRuntimes = nil
	d.ImportedModelMap = nil
	d.GPUPools = nil
	d.GPUNodeMap = nil
	d.GPUWorkloadMap = nil
	d.DedicatedAIClusterMap = nil
	d.InferenceServiceMap = nil
}

// mergeReloadedRepoData copies the repo-owned fields from fresh into d while
// preserving the lazily-loaded, k8s-backed fields already present in d
// (BaseModels, ServingRuntimes, ImportedModelMap, GPUPools, GPUNodeMap,
// GPUWorkloadMap, DedicatedAIClusterMap, InferenceServiceMap). It is used
// when a working-tree change triggers a dataset reload: LoadDataset
// repopulates only the repo-owned fields, so a wholesale assignment would
// wipe live k8s data. New repo-owned fields added to Dataset are carried
// across automatically; only the small, stable set of k8s fields is
// enumerated here (TestDataset_LiveFieldsDriftGuard keeps it complete).
func mergeReloadedRepoData(d, fresh *models.Dataset) {
	fresh.BaseModels = d.BaseModels
	fresh.ServingRuntimes = d.ServingRuntimes
	fresh.ImportedModelMap = d.ImportedModelMap
	fresh.GPUPools = d.GPUPools
	fresh.GPUNodeMap = d.GPUNodeMap
	fresh.GPUWorkloadMap = d.GPUWorkloadMap
	fresh.DedicatedAIClusterMap = d.DedicatedAIClusterMap
	fresh.InferenceServiceMap = d.InferenceServiceMap
	*d = *fresh
}
