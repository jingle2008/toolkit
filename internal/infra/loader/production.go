package loader

import (
	"context"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/internal/utils"
)

/*
ProductionLoader implements all loader interfaces using the production utils package.
*/
type ProductionLoader struct{}

/*
NOTE: The following ProductionLoader methods and their corresponding utils.* functions
must be updated to accept context.Context as the first parameter.
*/

// LoadDataset loads a dataset from the given repo and environment.
func (ProductionLoader) LoadDataset(ctx context.Context, repo string, env models.Environment) (*models.Dataset, error) {
	return utils.LoadDataset(ctx, repo, env) // TODO: Update utils.LoadDataset to accept context.Context
}

// LoadBaseModels loads base models from the given repo and environment.
func (ProductionLoader) LoadBaseModels(ctx context.Context, repo string, env models.Environment) (map[string]*models.BaseModel, error) {
	return utils.LoadBaseModels(ctx, repo, env) // TODO: Update utils.LoadBaseModels to accept context.Context
}

// LoadGpuPools loads GPU pools from the given repo and environment.
func (ProductionLoader) LoadGpuPools(ctx context.Context, repo string, env models.Environment) ([]models.GpuPool, error) {
	return utils.LoadGpuPools(ctx, repo, env) // TODO: Update utils.LoadGpuPools to accept context.Context
}

// LoadGpuNodes loads GPU nodes from the given kube config and environment.
func (ProductionLoader) LoadGpuNodes(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error) {
	return utils.LoadGpuNodes(ctx, kubeCfg, env) // TODO: Update utils.LoadGpuNodes to accept context.Context
}

// LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
func (ProductionLoader) LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return utils.LoadDedicatedAIClusters(ctx, kubeCfg, env) // TODO: Update utils.LoadDedicatedAIClusters to accept context.Context
}
