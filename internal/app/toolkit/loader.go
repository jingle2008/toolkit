package toolkit

import (
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

type Loader interface {
	LoadDataset(repo string, env models.Environment) (*models.Dataset, error)
	LoadBaseModels(repo string, env models.Environment) (map[string]*models.BaseModel, error)
	LoadGpuPools(repo string, env models.Environment) ([]models.GpuPool, error)
	LoadGpuNodes(kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error)
	LoadDedicatedAIClusters(kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error)
}

type ProductionLoader struct{}

func (ProductionLoader) LoadDataset(repo string, env models.Environment) (*models.Dataset, error) {
	return utils.LoadDataset(repo, env)
}

func (ProductionLoader) LoadBaseModels(repo string, env models.Environment) (map[string]*models.BaseModel, error) {
	return utils.LoadBaseModels(repo, env)
}

func (ProductionLoader) LoadGpuPools(repo string, env models.Environment) ([]models.GpuPool, error) {
	return utils.LoadGpuPools(repo, env)
}

func (ProductionLoader) LoadGpuNodes(kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error) {
	return utils.LoadGpuNodes(kubeCfg, env)
}

func (ProductionLoader) LoadDedicatedAIClusters(kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return utils.LoadDedicatedAIClusters(kubeCfg, env)
}
