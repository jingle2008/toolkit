// Package query loads the categories whose rows take more than one
// loader call — tenant-owned rows with their owners resolved, GPU pools
// with their OCI sizes — the same way for the CLI and MCP. A load
// returns its rows, warnings for what loaded only partly (the rows still
// show), and an error when the rows themselves didn't load. Each
// front-end shows the warnings its own way.
package query

import (
	"context"
	"errors"
	"fmt"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Source is where rows load from.
type Source struct {
	Loader     loader.Composite
	RepoPath   string
	KubeConfig string
}

// TenantDataset is env's tenants, for resolving tenant-owned rows. A
// failed load is a warning, not an error: the rows still show, keyed by
// raw tenancy-id with no owner.
func TenantDataset(ctx context.Context, src Source, env models.Environment) (*models.Dataset, []string) {
	ds, err := resolve.TenantDataset(ctx, src.Loader, src.RepoPath, env)
	if err != nil {
		return ds, []string{"tenant names unresolved: " + err.Error()}
	}
	return ds, nil
}

// DACs is env's dedicated AI clusters, re-keyed from raw tenancy-id to
// tenant name with each one's owner set; unresolved tenancies keep the
// raw key, and every DAC keeps its raw TenantID.
func DACs(ctx context.Context, src Source, env models.Environment) (map[string][]models.DedicatedAICluster, []string, error) {
	grouped, err := src.Loader.LoadDedicatedAIClusters(ctx, src.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load dedicated AI clusters: %w", err)
	}
	ds, warnings := TenantDataset(ctx, src, env)
	ds.SetDedicatedAIClusterMap(grouped)
	return ds.DedicatedAIClusterMap, warnings, nil
}

// ImportedModels is env's imported models, re-keyed and owned as DACs
// are.
func ImportedModels(ctx context.Context, src Source, env models.Environment) (map[string][]models.ImportedModel, []string, error) {
	grouped, err := src.Loader.LoadImportedModels(ctx, src.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load imported models: %w", err)
	}
	ds, warnings := TenantDataset(ctx, src, env)
	ds.SetImportedModelMap(grouped)
	return ds.ImportedModelMap, warnings, nil
}

// InferenceServices is env's InferenceServices, keyed by namespace, with
// their owners resolved in place.
func InferenceServices(ctx context.Context, src Source, env models.Environment) (map[string][]models.InferenceService, []string, error) {
	grouped, err := src.Loader.LoadInferenceServices(ctx, src.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load inference services: %w", err)
	}
	ds, warnings := TenantDataset(ctx, src, env)
	ds.SetInferenceServiceMap(grouped)
	return ds.InferenceServiceMap, warnings, nil
}

// GPUWorkloads is env's GPU workloads, keyed by node, with their owners
// resolved; the owners load while the workloads do. The DACs give a pod
// without a tenancy-id label its tenant, so a failed DAC load only leaves
// those pods unowned.
func GPUWorkloads(ctx context.Context, src Source, env models.Environment) (map[string][]models.GPUWorkload, []string, error) {
	type owners struct {
		ds       *models.Dataset
		warnings []string
	}
	ownersCh := make(chan owners, 1)
	go func() {
		ds, warnings := TenantDataset(ctx, src, env)
		if dacs, err := src.Loader.LoadDedicatedAIClusters(ctx, src.KubeConfig, env); err == nil {
			ds.SetDedicatedAIClusterMap(dacs)
		} else {
			warnings = append(warnings, "DACs for workload tenants unavailable: "+err.Error())
		}
		ownersCh <- owners{ds, warnings}
	}()
	grouped, err := src.Loader.LoadGPUWorkloadsByNode(ctx, src.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load gpu workloads: %w", err)
	}
	o := <-ownersCh
	o.ds.SetGPUWorkloadMap(grouped)
	return o.ds.GPUWorkloadMap, o.warnings, nil
}

// GPUPools is env's GPU pools from the Terraform repo, with ActualSize
// and Status from OCI. A source that failed to load is a warning, as is
// enrichment that didn't complete: the Terraform-derived columns still
// show.
func GPUPools(ctx context.Context, src Source, env models.Environment) ([]models.GPUPool, []string, error) {
	items, err := src.Loader.LoadGPUPools(ctx, src.RepoPath, env)
	var warnings []string
	if err != nil {
		partial, ok := errors.AsType[*terraform.PartialLoadError](err)
		if !ok {
			return nil, nil, fmt.Errorf("load gpu pools: %w", err)
		}
		for _, e := range partial.Errs {
			warnings = append(warnings, "load gpu pools: "+e.Error())
		}
	}
	if err := resolve.EnrichGPUPools(ctx, items, src.KubeConfig, env); err != nil {
		warnings = append(warnings, "gpu pool enrichment incomplete: "+err.Error())
	}
	return items, warnings, nil
}
