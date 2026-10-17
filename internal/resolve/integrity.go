package resolve

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

// IntegrityDataset loads every category the relations touch,
// concurrently, and resolves tenant owners as the TUI does: what
// `doctor --relations` checks, and what deciding faulty rows takes. A
// partial GPU-pool load is not fatal: poolNote describes it.
func IntegrityDataset(ctx context.Context, ld loader.Composite, repoPath, kubeConfig string, env models.Environment) (ds *models.Dataset, poolNote string, err error) {
	var (
		bms    []models.BaseModel
		srs    []models.ServingRuntime
		ims    map[string][]models.ImportedModel
		pools  []models.GPUPool
		nodes  map[string][]models.GPUNode
		pods   map[string][]models.GPUWorkload
		dacs   map[string][]models.DedicatedAICluster
		isvcs  map[string][]models.InferenceService
		repoDS *models.Dataset
	)
	g, gctx := errgroup.WithContext(ctx)
	load := func(what string, fn func() error) {
		g.Go(func() error {
			if err := fn(); err != nil {
				return fmt.Errorf("load %s: %w", what, err)
			}
			return nil
		})
	}
	load("dataset", func() (err error) { repoDS, err = ld.LoadDataset(gctx, repoPath, env); return })
	load("base models", func() (err error) { bms, err = ld.LoadBaseModels(gctx, kubeConfig, env); return })
	load("serving runtimes", func() (err error) { srs, err = ld.LoadServingRuntimes(gctx, kubeConfig, env); return })
	load("imported models", func() (err error) { ims, err = ld.LoadImportedModels(gctx, kubeConfig, env); return })
	load("gpu nodes", func() (err error) { nodes, err = ld.LoadGPUNodesByPool(gctx, kubeConfig, env); return })
	load("gpu workloads", func() (err error) { pods, err = ld.LoadGPUWorkloadsByNode(gctx, kubeConfig, env); return })
	load("dedicated AI clusters", func() (err error) { dacs, err = ld.LoadDedicatedAIClusters(gctx, kubeConfig, env); return })
	load("inference services", func() (err error) { isvcs, err = ld.LoadInferenceServices(gctx, kubeConfig, env); return })
	load("gpu pools", func() error {
		var err error
		pools, err = ld.LoadGPUPools(gctx, repoPath, env)
		if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
			poolNote = "some GPU pool sources failed to load: " + partial.Error()
			return nil
		}
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, "", err
	}

	ds = repoDS
	ds.BaseModels, ds.ServingRuntimes, ds.GPUPools, ds.GPUNodeMap = bms, srs, pools, nodes
	ds.SetDedicatedAIClusterMap(dacs)
	ds.SetImportedModelMap(ims)
	ds.SetGPUWorkloadMap(pods)
	ds.SetInferenceServiceMap(isvcs)
	return ds, poolNote, nil
}

// enrichGPUPoolsFn is the seam tests use to fake the OCI pool lookup.
var enrichGPUPoolsFn = EnrichGPUPools

// FaultyDataset loads what deciding category's faulty rows takes
// (relations.FaultyRows): every category, so a row naming a missing
// parent is caught, and for GPUPool the pools' live sizes from OCI. A
// pool is faulty when its live size differs from Terraform's, so without
// the live sizes every pool would look faulty: that is an error, not a
// warning. A partial GPU-pool load is a warning.
func FaultyDataset(ctx context.Context, ld loader.Composite, repoPath, kubeConfig string, env models.Environment, category domain.Category) (*models.Dataset, []string, error) {
	ds, poolNote, err := IntegrityDataset(ctx, ld, repoPath, kubeConfig, env)
	if err != nil {
		return nil, nil, err
	}
	var warnings []string
	if poolNote != "" {
		warnings = append(warnings, poolNote)
	}
	if category == domain.GPUPool {
		if err := enrichGPUPoolsFn(ctx, ds.GPUPools, kubeConfig, env); err != nil {
			return nil, nil, fmt.Errorf("live GPU pool sizes unavailable, so faulty pools can't be told apart: %w", err)
		}
	}
	return ds, warnings, nil
}
