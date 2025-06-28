/*
Package production provides the production Loader implementation for the toolkit application.
*/
package production

import (
	"context"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
Loader implements all loader interfaces using the production utils package.
*/
// Loader implements all loader interfaces using the production utils package.
type Loader struct {
	// Add dependencies here for future extensibility, e.g.:
	// K8sFactory func(kubeCfg, context string) (k8s.Helper, error)
}

// NewLoader returns a Loader implementation for production use.
func NewLoader() loader.Loader {
	return &Loader{}
}

/*
NOTE: The following ProductionLoader methods and their corresponding utils.* functions
must be updated to accept context.Context as the first parameter.
*/

/*
LoadDataset loads a dataset from the given repo and environment.
*/
func (Loader) LoadDataset(ctx context.Context, repo string, env models.Environment) (*models.Dataset, error) {
	return configloader.LoadDataset(ctx, repo, env)
}

// LoadBaseModels loads base models from the given repo and environment.
func (Loader) LoadBaseModels(ctx context.Context, repo string, env models.Environment) (map[string]*models.BaseModel, error) {
	return terraform.LoadBaseModels(ctx, repo, env)
}

// LoadGpuPools loads GPU pools from the given repo and environment.
func (Loader) LoadGpuPools(ctx context.Context, repo string, env models.Environment) ([]models.GpuPool, error) {
	return terraform.LoadGpuPools(ctx, repo, env)
}

/*
LoadGpuNodes loads GPU nodes from the given kube config and environment.
Implements the Loader interface but is not yet migrated.
*/
func (Loader) LoadGpuNodes(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GpuNode, error) {
	client, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.GetKubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadGpuNodes(ctx, client)
}

/*
LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
Implements the Loader interface but is not yet migrated.
*/
func (Loader) LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.GetKubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadDedicatedAIClusters(ctx, client)
}

// LoadTenancyOverrideGroup loads tenants and all tenancy override maps for a given realm.
func (Loader) LoadTenancyOverrideGroup(ctx context.Context, repo string, env models.Environment) (models.TenancyOverrideGroup, error) {
	return configloader.LoadTenancyOverrideGroup(ctx, repo, env.Realm)
}

/*
LoadLimitRegionalOverrides ...
*/
func (Loader) LoadLimitRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.LimitRegionalOverride, error) {
	return configloader.LoadLimitRegionalOverrides(ctx, repo, env.Realm)
}

// LoadConsolePropertyRegionalOverrides loads console property regional overrides for the given repo and environment.
func (Loader) LoadConsolePropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return configloader.LoadConsolePropertyRegionalOverrides(ctx, repo, env.Realm)
}

// LoadPropertyRegionalOverrides loads property regional overrides for the given repo and environment.
func (Loader) LoadPropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.PropertyRegionalOverride, error) {
	return configloader.LoadPropertyRegionalOverrides(ctx, repo, env.Realm)
}
