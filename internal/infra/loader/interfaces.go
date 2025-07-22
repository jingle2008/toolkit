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
GpuPoolLoader defines an interface for loading GPU pools.
*/
type GpuPoolLoader interface {
	// LoadGpuPools loads GPU pools from the given repo and environment.
	LoadGpuPools(ctx context.Context, repo string, env models.Environment) ([]models.GpuPool, error)
}

/*
GpuNodeLoader defines an interface for loading GPU nodes.
*/
type GpuNodeLoader interface {
	// LoadGpuNodes loads GPU nodes from the given kube config and environment.
	LoadGpuNodes(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error)
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
Loader is a composite interface that embeds all loader interfaces.
*/
type Loader interface {
	DatasetLoader
	BaseModelLoader
	GpuPoolLoader
	GpuNodeLoader
	DedicatedAIClusterLoader
	TenancyOverrideLoader
	RegionalOverrideLoader
}
