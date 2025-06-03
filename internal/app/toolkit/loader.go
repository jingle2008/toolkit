package toolkit

import (
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

/*
DatasetLoader defines an interface for loading datasets.
*/
type DatasetLoader interface {
	// LoadDataset loads a dataset from the given repo and environment.
	LoadDataset(repo string, env models.Environment) (*models.Dataset, error)
}

/*
BaseModelLoader defines an interface for loading base models.
*/
type BaseModelLoader interface {
	// LoadBaseModels loads base models from the given repo and environment.
	LoadBaseModels(repo string, env models.Environment) (map[string]*models.BaseModel, error)
}

/*
GpuPoolLoader defines an interface for loading GPU pools.
*/
type GpuPoolLoader interface {
	// LoadGpuPools loads GPU pools from the given repo and environment.
	LoadGpuPools(repo string, env models.Environment) ([]models.GpuPool, error)
}

/*
GpuNodeLoader defines an interface for loading GPU nodes.
*/
type GpuNodeLoader interface {
	// LoadGpuNodes loads GPU nodes from the given kube config and environment.
	LoadGpuNodes(kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error)
}

/*
DedicatedAIClusterLoader defines an interface for loading dedicated AI clusters.
*/
type DedicatedAIClusterLoader interface {
	// LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
	LoadDedicatedAIClusters(kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error)
}

/*
ProductionLoader implements all loader interfaces using the production utils package.
*/
type ProductionLoader struct{}

// LoadDataset loads a dataset from the given repo and environment.
func (ProductionLoader) LoadDataset(repo string, env models.Environment) (*models.Dataset, error) {
	return utils.LoadDataset(repo, env)
}

// LoadBaseModels loads base models from the given repo and environment.
func (ProductionLoader) LoadBaseModels(repo string, env models.Environment) (map[string]*models.BaseModel, error) {
	return utils.LoadBaseModels(repo, env)
}

// LoadGpuPools loads GPU pools from the given repo and environment.
func (ProductionLoader) LoadGpuPools(repo string, env models.Environment) ([]models.GpuPool, error) {
	return utils.LoadGpuPools(repo, env)
}

// LoadGpuNodes loads GPU nodes from the given kube config and environment.
func (ProductionLoader) LoadGpuNodes(kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error) {
	return utils.LoadGpuNodes(kubeCfg, env)
}

// LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
func (ProductionLoader) LoadDedicatedAIClusters(kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return utils.LoadDedicatedAIClusters(kubeCfg, env)
}
