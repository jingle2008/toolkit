/*
Package production provides the production Client implementation for the toolkit application.
*/
package production

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Compile-time guard: *Client must satisfy the optional writer
// interface. If UpsertTenantMetadata is ever changed to a value
// receiver, New's &Client return would no longer carry it where this
// matters — this line keeps that a compile error, not a runtime one.
var _ loader.TenantMetadataWriter = (*Client)(nil)

/*
Client implements all loader interfaces using the production utils package.
*/
type Client struct {
	metadataFile    string
	metadata        *models.Metadata
	metadataLoadErr error // non-nil when an EXISTING metadata file failed to parse; blocks writes to avoid clobbering it
}

// New returns a Client implementation for production use.
func New(ctx context.Context, metadataFile string) loader.Composite {
	l := &Client{
		metadataFile: metadataFile,
		metadata:     &models.Metadata{},
	}

	if metadataFile != "" {
		if _, statErr := os.Stat(metadataFile); statErr == nil {
			// File exists: a parse failure must NOT be swallowed, or the
			// first write would overwrite the user's existing entries.
			m, err := configloader.LoadMetadata(metadataFile)
			if err != nil {
				l.metadataLoadErr = fmt.Errorf("existing metadata file %s failed to load: %w", metadataFile, err)
				logging.FromContext(ctx).Errorw("failed to load metadata file", "file", metadataFile, "error", err)
			} else {
				l.metadata = m
			}
		}
		// Missing file: leave empty metadata; the first save creates it.
	}
	return l
}

// MetadataPath returns the configured metadata file path (for display).
func (l *Client) MetadataPath() string { return l.metadataFile }

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

// LoadGPUWorkloadsByNode lists GPU-consuming pods grouped by node.
func (Client) LoadGPUWorkloadsByNode(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUWorkload, error) {
	client, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadGPUWorkloadsByNode(ctx, client)
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

// UpsertTenantMetadata merges entry into the metadata file (replacing
// any entry with the same ID, else appending) and persists it,
// creating the file if absent. The in-memory metadata is updated only
// after the write succeeds, so a failed write leaves memory and disk
// consistent (the caller's error is the single source of truth).
//
// Pointer receiver: it reassigns l.metadata, so the runtime type
// behind a loader.Composite must be *Client for the optional
// loader.TenantMetadataWriter assertion to succeed (production.New
// returns &Client{...}, so it does).
//
// Not safe for concurrent use with the Load* methods: they share the
// same *models.Metadata. Callers must sequence the write before any
// concurrent read (the TUI dispatches the save, waits for its result,
// then triggers the reload).
func (l *Client) UpsertTenantMetadata(entry models.TenantMetadata) error {
	if l.metadataFile == "" {
		return errors.New("no metadata file configured")
	}
	if l.metadataLoadErr != nil {
		return fmt.Errorf("refusing to overwrite metadata: %w", l.metadataLoadErr)
	}
	next := &models.Metadata{}
	if l.metadata != nil {
		next.Tenants = make([]models.TenantMetadata, len(l.metadata.Tenants))
		copy(next.Tenants, l.metadata.Tenants)
	}
	configloader.UpsertTenant(next, entry)
	if err := configloader.SaveMetadata(l.metadataFile, next); err != nil {
		return err
	}
	l.metadata = next
	return nil
}

// Compile-time guard: *Client must satisfy the optional Watcher
// interface, kept out of Composite (see loader.Watcher docs).
var _ loader.Watcher = (*Client)(nil)

// WatchBaseModels establishes a watch on ClusterBaseModel CRs.
func (Client) WatchBaseModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchBaseModels(ctx, client)
}

// WatchImportedModels establishes a watch on the imported-model sources.
func (Client) WatchImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchImportedModels(ctx, client)
}

// WatchGPUNodes establishes a watch on GPU nodes and GPU pods.
func (Client) WatchGPUNodes(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchGPUNodes(ctx, cs)
}

// WatchGPUWorkloads establishes a watch on GPU pods.
func (Client) WatchGPUWorkloads(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchGPUWorkloads(ctx, cs)
}

// WatchDedicatedAIClusters establishes a watch on DAC CRs and GPU pods.
func (Client) WatchDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	kubeCtx := env.KubeContext()
	dyn, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, kubeCtx)
	if err != nil {
		return nil, err
	}
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, kubeCtx)
	if err != nil {
		return nil, err
	}
	return k8s.WatchDedicatedAIClusters(ctx, dyn, cs)
}
