// Package loader defines interfaces for loading datasets and related resources in the toolkit application.
package loader

import (
	"context"

	"github.com/jingle2008/toolkit/pkg/models"
)

/*
DatasetLoader defines an interface for loading datasets.
*/
type DatasetLoader interface {
	// LoadDataset loads a dataset from the given repo and environment.
	LoadDataset(ctx context.Context, repo string, env models.Environment) (*models.Dataset, error)
}

/*
BaseModelLoader defines an interface for loading base models.
*/
type BaseModelLoader interface {
	// LoadBaseModels loads base models from the given repo and environment.
	LoadBaseModels(ctx context.Context, repo string, env models.Environment) ([]models.BaseModel, error)
}

/*
ImportedModelLoader defines an interface for loading tenant-imported
models (namespaced BaseModel CRs + ClusterBaseModel CRs with a
`tenancy-id` label).
*/
type ImportedModelLoader interface {
	// LoadImportedModels loads imported models from the given kube config and environment.
	// Returns a tenant-keyed map (raw TenantID, or `"UNKNOWN_TENANCY"` for orphans);
	// re-keying by Tenant.Name is handled by Dataset.SetImportedModelMap.
	LoadImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.ImportedModel, error)
}

/*
GPUPoolLoader defines an interface for loading GPU pools.
*/
type GPUPoolLoader interface {
	// LoadGPUPools loads GPU pools from the given repo and environment.
	LoadGPUPools(ctx context.Context, repo string, env models.Environment) ([]models.GPUPool, error)
}

/*
GPUNodeLoader defines an interface for loading GPU nodes.
*/
type GPUNodeLoader interface {
	// LoadGPUNodesByPool loads GPU nodes from the given kube config and environment.
	LoadGPUNodesByPool(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUNode, error)
}

// GPUWorkloadLoader loads GPU-consuming pods grouped by node.
type GPUWorkloadLoader interface {
	LoadGPUWorkloadsByNode(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUWorkload, error)
}

/*
DedicatedAIClusterLoader defines an interface for loading dedicated AI clusters.
*/
type DedicatedAIClusterLoader interface {
	// LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
	LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error)
}

/*
TenancyOverrideLoader defines methods for loading tenancy override maps.
*/
type TenancyOverrideLoader interface {
	LoadTenancyOverrideGroup(ctx context.Context, repo string, env models.Environment) (models.TenancyOverrideGroup, error)
}

/*
RegionalOverrideLoader defines methods for loading regional override slices.
*/
type RegionalOverrideLoader interface {
	LoadLimitRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.LimitRegionalOverride, error)
	LoadConsolePropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.ConsolePropertyRegionalOverride, error)
	LoadPropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.PropertyRegionalOverride, error)
}

/*
Composite is a composite interface that embeds all loader interfaces.
*/
type Composite interface {
	DatasetLoader
	BaseModelLoader
	ImportedModelLoader
	GPUPoolLoader
	GPUNodeLoader
	GPUWorkloadLoader
	DedicatedAIClusterLoader
	TenancyOverrideLoader
	RegionalOverrideLoader
}

/*
TenantMetadataWriter is an OPTIONAL capability: persisting a tenant
metadata entry to the backing metadata file. It is deliberately kept
out of Composite so the many fake loaders used in tests need not
implement it. Callers type-assert a Composite to this interface and
degrade gracefully when the assertion fails.
*/
type TenantMetadataWriter interface {
	// UpsertTenantMetadata merges entry into the metadata file
	// (replacing any entry with the same ID, else appending) and
	// persists it, creating the file if it does not exist.
	UpsertTenantMetadata(entry models.TenantMetadata) error
}

/*
Watcher is an OPTIONAL capability: establishing Kubernetes watches that
emit a coalesced "reload now" signal for the k8s-backed categories. Like
TenantMetadataWriter it is deliberately kept out of Composite so the many
fake loaders used in tests need not implement it. Callers type-assert a
Composite to this interface and fall back to a one-shot load when the
assertion fails or a method returns an error.

Each method returns a channel that yields one value whenever the
category's underlying resources change (debounced). The caller owns ctx;
cancelling it stops the watch and closes the channel. The channel also
closes if the stream dies, which the caller treats as a fallback signal.
*/
type Watcher interface {
	WatchBaseModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchGPUNodes(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchGPUWorkloads(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
}

/*
RepoWatcher is an OPTIONAL capability: establishing a filesystem watch on the
repo working tree that emits a coalesced "reload now" signal, making
repo-backed categories live the way Watcher makes k8s-backed categories live.
Like Watcher and TenantMetadataWriter it is deliberately kept out of Composite
so the many fake loaders used in tests need not implement it. Callers
type-assert a Composite to this interface and fall back to a static load when
the assertion fails or the method returns an error.

The returned channel yields one value whenever any non-hidden file under
repoPath changes (debounced). The caller owns ctx; cancelling it stops the
watch and closes the channel. The channel also closes if the watcher dies,
which the caller treats as a fallback signal.
*/
type RepoWatcher interface {
	WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error)
}
