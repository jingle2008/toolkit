package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/output"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// faultyDatasetFn is the seam tests use to fake the full load --faulty
// needs.
var faultyDatasetFn = resolve.FaultyDataset

// validateFaulty checks `get --faulty` can run for cat (nothing to check
// when faulty is off): only some categories have faulty rows, and
// deciding takes the cluster, whatever cat itself needs.
func validateFaulty(cfg config.Config, cat domain.Category, faulty bool) error {
	if !faulty {
		return nil
	}
	if err := relations.FaultyCategoryErr(cat); err != nil {
		return fmt.Errorf("--faulty: %w", err)
	}
	if cfg.KubeConfig == "" {
		return fmt.Errorf("--faulty needs --kubeconfig: a row naming a missing parent is found by loading every category")
	}
	if _, err := os.Stat(cfg.KubeConfig); err != nil {
		return fmt.Errorf("kubeconfig %q not readable: %w", cfg.KubeConfig, err)
	}
	return nil
}

// emitFaulty is emitCategory for `get --faulty`: only cat's faulty rows,
// the ones the TUI's Ctrl+Z shows (relations.FaultyRows). It loads every
// category, so a row naming a missing parent is caught; --filter,
// --related-to, --limit, -o and --columns apply as usual.
func emitFaulty(
	ctx context.Context,
	w writer,
	ld loader.Composite,
	cat domain.Category,
	cfg config.Config,
	env models.Environment,
	filter string,
	related *domain.Scope,
	limit int,
	opts output.Options,
	selected []string,
) error {
	ds, warnings, err := faultyDatasetFn(ctx, ld, cfg.RepoPath, cfg.KubeConfig, env, cat)
	if err != nil {
		return fmt.Errorf("--faulty: %w", err)
	}
	for _, msg := range warnings {
		logging.FromContext(ctx).Warn("get --faulty", "warning", msg)
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	switch cat { //nolint:exhaustive // validateFaulty admits only relations.FaultyCategories
	case domain.BaseModel:
		return writeSlice(w, faultySlice(ds, cat, ds.BaseModels, filter), limit, opts, cat, env, selected)
	case domain.ServingRuntime:
		return writeSlice(w, faultySlice(ds, cat, ds.ServingRuntimes, filter), limit, opts, cat, env, selected)
	case domain.Tenant:
		return writeSlice(w, faultySlice(ds, cat, ds.Tenants, filter), limit, opts, cat, env, selected)
	case domain.GPUPool:
		return writeSlice(w, faultySlice(ds, cat, ds.GPUPools, filter), limit, opts, cat, env, selected)
	case domain.GPUNode:
		return writeMap(w, relatedOrFiltered(relations.GPUNode, relations.FaultyMap(ds, cat, ds.GPUNodeMap), cat, related, filter),
			limit, opts, cat, env, selected)
	case domain.GPUWorkload:
		return writeMap(w, relatedOrFiltered(relations.GPUWorkload, relations.FaultyMap(ds, cat, ds.GPUWorkloadMap), cat, related, filter),
			limit, opts, cat, env, selected)
	case domain.DedicatedAICluster:
		return writeMap(w, relatedOrFiltered(relations.DedicatedAICluster, relations.FaultyMap(ds, cat, ds.DedicatedAIClusterMap), cat, related, filter),
			limit, opts, cat, env, selected)
	case domain.InferenceService:
		return writeMap(w, relatedOrFiltered(relations.InferenceService, relations.FaultyMap(ds, cat, ds.InferenceServiceMap), cat, related, filter),
			limit, opts, cat, env, selected)
	}
	return fmt.Errorf("--faulty: %s rows are never faulty", cat)
}

// faultySlice keeps a flat category's faulty rows that match filter.
func faultySlice[T models.NamedFilterable](ds *models.Dataset, cat domain.Category, items []T, filter string) []T {
	return collections.FilterSlice(relations.FaultySlice(ds, cat, items), nil, filter, nil)
}
