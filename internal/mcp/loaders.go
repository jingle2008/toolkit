package mcp

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/pkg/models"
)

// The grouped categories' loads, as their list tools show them (tenant
// owners resolved, re-keyed by tenant name where the TUI does) before
// flattening: the list tools, list_related and get_item share them, so a
// row looks the same from each. Errors name what failed to load.

func (s *Server) importedModels(ctx context.Context, env models.Environment) (map[string][]models.ImportedModel, []string, error) {
	grouped, err := s.loader.LoadImportedModels(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load imported models: %w", err)
	}
	// Each item carries its own `tenantId`; resolution adds `owner` and
	// lets the filter match the tenant name (the re-keyed group key).
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetImportedModelMap(grouped)
	return ds.ImportedModelMap, warnings, nil
}

func (s *Server) gpuNodes(ctx context.Context, env models.Environment) (map[string][]models.GPUNode, error) {
	grouped, err := s.loader.LoadGPUNodesByPool(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, fmt.Errorf("load gpu nodes: %w", err)
	}
	return grouped, nil
}

func (s *Server) gpuWorkloads(ctx context.Context, env models.Environment) (map[string][]models.GPUWorkload, []string, error) {
	grouped, err := s.loader.LoadGPUWorkloadsByNode(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load gpu workloads: %w", err)
	}
	ds, warnings := s.tenantDataset(ctx, env)
	// The DACs give a pod without a tenancy-id label its tenant; a failed
	// load only leaves those pods unowned.
	if dacs, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, env); err == nil {
		ds.SetDedicatedAIClusterMap(dacs)
	} else {
		s.logger.Warnw("dacs for workload tenants unavailable", "error", err)
	}
	ds.SetGPUWorkloadMap(grouped)
	return ds.GPUWorkloadMap, warnings, nil
}

func (s *Server) dacs(ctx context.Context, env models.Environment) (map[string][]models.DedicatedAICluster, []string, error) {
	grouped, err := s.loader.LoadDedicatedAIClusters(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load dedicated AI clusters: %w", err)
	}
	// Re-keyed by tenant name (so the filter matches it) with `owner` set;
	// each value keeps its raw `tenantId`.
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetDedicatedAIClusterMap(grouped)
	return ds.DedicatedAIClusterMap, warnings, nil
}

func (s *Server) inferenceServices(ctx context.Context, env models.Environment) (map[string][]models.InferenceService, []string, error) {
	grouped, err := s.loader.LoadInferenceServices(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, nil, fmt.Errorf("load inference services: %w", err)
	}
	ds, warnings := s.tenantDataset(ctx, env)
	ds.SetInferenceServiceMap(grouped)
	return ds.InferenceServiceMap, warnings, nil
}

func (s *Server) modelArtifacts(ctx context.Context, env models.Environment) (map[string][]models.ModelArtifact, error) {
	ds, err := s.loader.LoadDataset(ctx, s.cfg.RepoPath, env)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}
	return ds.ModelArtifactMap, nil
}

func (s *Server) tenancyGroup(ctx context.Context, env models.Environment) (models.TenancyOverrideGroup, error) {
	grp, err := s.loader.LoadTenancyOverrideGroup(ctx, s.cfg.RepoPath, env)
	if err != nil {
		return models.TenancyOverrideGroup{}, fmt.Errorf("load tenancy override group: %w", err)
	}
	return grp, nil
}
