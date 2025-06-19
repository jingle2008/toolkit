package loader

import (
	"context"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
ProductionLoader implements all loader interfaces using the production utils package.
*/
type ProductionLoader struct{}

/*
NOTE: The following ProductionLoader methods and their corresponding utils.* functions
must be updated to accept context.Context as the first parameter.
*/

/*
LoadDataset loads a dataset from the given repo and environment.
*/
func (ProductionLoader) LoadDataset(ctx context.Context, repo string, env models.Environment) (*models.Dataset, error) {
	return configloader.LoadDataset(ctx, repo, env)
}

// LoadBaseModels loads base models from the given repo and environment.
func (ProductionLoader) LoadBaseModels(ctx context.Context, repo string, env models.Environment) (map[string]*models.BaseModel, error) {
	return terraform.LoadBaseModels(ctx, repo, env)
}

// LoadGpuPools loads GPU pools from the given repo and environment.
func (ProductionLoader) LoadGpuPools(ctx context.Context, repo string, env models.Environment) ([]models.GpuPool, error) {
	return terraform.LoadGpuPools(ctx, repo, env)
}

/*
LoadGpuNodes loads GPU nodes from the given kube config and environment.
Implements the Loader interface but is not yet migrated.
*/
func (ProductionLoader) LoadGpuNodes(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error) {
	helper, err := k8s.NewHelper(kubeCfg, env.GetKubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadGpuNodes(ctx, helper)
}

/*
LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
Implements the Loader interface but is not yet migrated.
*/
func (ProductionLoader) LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	helper, err := k8s.NewHelper(kubeCfg, env.GetKubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadDedicatedAIClusters(ctx, helper)
}

// LoadTenancyOverrideGroup loads tenants and all tenancy override maps for a given realm.
func (ProductionLoader) LoadTenancyOverrideGroup(ctx context.Context, repo string, env models.Environment) (models.TenancyOverrideGroup, error) {
	return configloader.LoadTenancyOverrideGroup(ctx, repo, env.Realm)
}

/*
LoadLimitRegionalOverrides ...
*/
func (ProductionLoader) LoadLimitRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.LimitRegionalOverride, error) {
	return configloader.LoadLimitRegionalOverrides(ctx, repo, env.Realm)
}

// LoadConsolePropertyRegionalOverrides loads console property regional overrides for the given repo and environment.
func (ProductionLoader) LoadConsolePropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return configloader.LoadConsolePropertyRegionalOverrides(ctx, repo, env.Realm)
}

// LoadPropertyRegionalOverrides loads property regional overrides for the given repo and environment.
func (ProductionLoader) LoadPropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.PropertyRegionalOverride, error) {
	return configloader.LoadPropertyRegionalOverrides(ctx, repo, env.Realm)
}
