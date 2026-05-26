/*
Package production provides the production Client implementation for the toolkit application.
*/
package production

import (
	"context"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
Client implements all loader interfaces using the production utils package.
*/
type Client struct {
	metadataFile string
	metadata     *models.Metadata
}

// New returns a Client implementation for production use.
func New(ctx context.Context, metadataFile string) loader.Composite {
	l := &Client{
		metadataFile: metadataFile,
		metadata:     &models.Metadata{},
	}

	if metadataFile != "" {
		m, err := configloader.LoadMetadata(metadataFile)
		if err != nil {
			logging.FromContext(ctx).Errorw("failed to load metadata file", "file", metadataFile, "error", err)
		} else {
			l.metadata = m
		}
	}
	return l
}

/*
LoadDataset loads a dataset from the given repo and environment.
*/
func (l Client) LoadDataset(ctx context.Context, repo string, env models.Environment) (*models.Dataset, error) {
	return configloader.LoadDataset(ctx, repo, env, l.metadata)
}

/*
LoadBaseModels loads base models from the cluster using the provided kubeconfig and environment.
*/
func (Client) LoadBaseModels(ctx context.Context, kubeCfg string, env models.Environment) ([]models.BaseModel, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadBaseModels(ctx, client)
}

// LoadImportedModels loads tenant-imported models from the cluster
// (namespaced BaseModel CRs + ClusterBaseModel CRs with a
// `tenancy-id` label) grouped by raw TenantID, using the provided
// kubeconfig and environment.
func (Client) LoadImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.ImportedModel, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadImportedModels(ctx, client)
}

// LoadGPUPools loads GPU pools from the given repo and environment.
func (Client) LoadGPUPools(ctx context.Context, repo string, env models.Environment) ([]models.GPUPool, error) {
	return terraform.LoadGPUPools(ctx, repo, env)
}

// LoadGPUNodesByPool loads GPU nodes from the given kube config and environment.
func (Client) LoadGPUNodesByPool(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUNode, error) {
	client, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadGPUNodesByPool(ctx, client)
}

// LoadDedicatedAIClusters loads dedicated AI clusters from the given kube config and environment.
func (Client) LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadDedicatedAIClusters(ctx, client)
}

// LoadTenancyOverrideGroup loads tenants and all tenancy override maps for a given realm.
func (l Client) LoadTenancyOverrideGroup(ctx context.Context, repo string, env models.Environment) (models.TenancyOverrideGroup, error) {
	return configloader.LoadTenancyOverrideGroup(ctx, repo, env.Realm, l.metadata)
}

/*
LoadLimitRegionalOverrides ...
*/
func (Client) LoadLimitRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.LimitRegionalOverride, error) {
	return configloader.LoadLimitRegionalOverrides(ctx, repo, env.Realm)
}

// LoadConsolePropertyRegionalOverrides loads console property regional overrides for the given repo and environment.
func (Client) LoadConsolePropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return configloader.LoadConsolePropertyRegionalOverrides(ctx, repo, env.Realm)
}

// LoadPropertyRegionalOverrides loads property regional overrides for the given repo and environment.
func (Client) LoadPropertyRegionalOverrides(ctx context.Context, repo string, env models.Environment) ([]models.PropertyRegionalOverride, error) {
	return configloader.LoadPropertyRegionalOverrides(ctx, repo, env.Realm)
}
