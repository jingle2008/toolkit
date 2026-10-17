package resolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/pkg/models"
)

// MetricsTarget finds category's row named name (exactly, or
// <group>/<name> when groups share it, as `get <category> <name>`), with
// tenant owners resolved as listed, and loads the model catalog its
// dashboard plan needs (telemetry.CatalogFor). `toolkit metrics` and MCP's
// metrics_url both go through it, then telemetry.PlanFor. warnings are
// non-fatal (tenant names unresolved).
func MetricsTarget(
	ctx context.Context, ld loader.Composite, repoPath, kubeConfig string, env models.Environment, category domain.Category, name string,
) (item any, catalog *models.Dataset, warnings []string, err error) {
	ds, err := TenantDataset(ctx, ld, repoPath, env)
	if err != nil {
		warnings = append(warnings, "tenant names unresolved: "+err.Error())
	}
	item, err = metricsRow(ctx, ld, kubeConfig, env, ds, category, name)
	if err != nil {
		return nil, nil, nil, err
	}
	catalog, err = metricsCatalog(ctx, ld, kubeConfig, env, item)
	if err != nil {
		return nil, nil, nil, err
	}
	return item, catalog, warnings, nil
}

// pickPtr picks the row named name and returns a pointer to it, the form
// telemetry.PlanFor takes.
func pickPtr[T any](grouped map[string][]T, name, what string) (any, error) {
	_, it, err := collections.PickByNameGrouped(grouped, name, what)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// metricsCatalog loads the model catalog item's plan needs, or none.
func metricsCatalog(ctx context.Context, ld loader.Composite, kubeConfig string, env models.Environment, item any) (*models.Dataset, error) {
	cat, need := telemetry.CatalogFor(item)
	ds := &models.Dataset{}
	if !need {
		return ds, nil
	}
	if cat == domain.ImportedModel {
		grouped, err := ld.LoadImportedModels(ctx, kubeConfig, env)
		if err != nil {
			return nil, fmt.Errorf("load imported models: %w", err)
		}
		ds.ImportedModelMap = grouped
		return ds, nil
	}
	bms, err := ld.LoadBaseModels(ctx, kubeConfig, env)
	if err != nil {
		return nil, fmt.Errorf("load base models: %w", err)
	}
	ds.BaseModels = bms
	return ds, nil
}

// metricsRow loads category's rows into ds (owners resolved) and picks
// the one named name.
func metricsRow(
	ctx context.Context, ld loader.Composite, kubeConfig string, env models.Environment, ds *models.Dataset, category domain.Category, name string,
) (any, error) {
	switch category { //nolint:exhaustive // the categories with dashboards; others error below
	case domain.DedicatedAICluster:
		grouped, lerr := ld.LoadDedicatedAIClusters(ctx, kubeConfig, env)
		if lerr != nil {
			return nil, fmt.Errorf("load dedicated AI clusters: %w", lerr)
		}
		ds.SetDedicatedAIClusterMap(grouped)
		return pickPtr(ds.DedicatedAIClusterMap, name, category.String())
	case domain.ImportedModel:
		grouped, lerr := ld.LoadImportedModels(ctx, kubeConfig, env)
		if lerr != nil {
			return nil, fmt.Errorf("load imported models: %w", lerr)
		}
		ds.SetImportedModelMap(grouped)
		return pickPtr(ds.ImportedModelMap, name, category.String())
	case domain.GPUWorkload:
		grouped, lerr := ld.LoadGPUWorkloadsByNode(ctx, kubeConfig, env)
		if lerr != nil {
			return nil, fmt.Errorf("load gpu workloads: %w", lerr)
		}
		ds.SetGPUWorkloadMap(grouped)
		return pickPtr(ds.GPUWorkloadMap, name, category.String())
	case domain.InferenceService:
		grouped, lerr := ld.LoadInferenceServices(ctx, kubeConfig, env)
		if lerr != nil {
			return nil, fmt.Errorf("load inference services: %w", lerr)
		}
		ds.SetInferenceServiceMap(grouped)
		return pickPtr(ds.InferenceServiceMap, name, category.String())
	default:
		names := make([]string, len(telemetry.MetricsCategories))
		for i, c := range telemetry.MetricsCategories {
			names[i] = strings.ToLower(c.String())
		}
		return nil, fmt.Errorf("%s rows have no metrics dashboard; metrics are for: %s", category, strings.Join(names, ", "))
	}
}
