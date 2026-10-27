package mcp

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/internal/query"
	"github.com/jingle2008/toolkit/pkg/models"
)

// The grouped categories' loads, as their list tools show them (tenant
// owners resolved, re-keyed by tenant name where the TUI does) before
// flattening: the list tools, list_related and get_item share them, so a
// row looks the same from each. Errors name what failed to load.

// source is where the server's rows load from.
func (s *Server) source() query.Source {
	return query.Source{Loader: s.loader, RepoPath: s.cfg.RepoPath, KubeConfig: s.cfg.KubeConfig}
}

// logLoadWarnings logs what loaded only partly, for operators reading the
// log file; the client sees the same warnings in listResult.Warnings.
func (s *Server) logLoadWarnings(what string, warnings []string) {
	for _, w := range warnings {
		s.logger.Warn("load incomplete", "surface", "mcp", "what", what, "warning", w)
	}
}

func (s *Server) importedModels(ctx context.Context, env models.Environment) (map[string][]models.ImportedModel, []string, error) {
	grouped, warnings, err := query.ImportedModels(ctx, s.source(), env)
	s.logLoadWarnings("imported models", warnings)
	return grouped, warnings, err
}

func (s *Server) gpuNodes(ctx context.Context, env models.Environment) (map[string][]models.GPUNode, error) {
	grouped, err := s.loader.LoadGPUNodesByPool(ctx, s.cfg.KubeConfig, env)
	if err != nil {
		return nil, fmt.Errorf("load gpu nodes: %w", err)
	}
	return grouped, nil
}

func (s *Server) gpuWorkloads(ctx context.Context, env models.Environment) (map[string][]models.GPUWorkload, []string, error) {
	grouped, warnings, err := query.GPUWorkloads(ctx, s.source(), env)
	s.logLoadWarnings("gpu workloads", warnings)
	return grouped, warnings, err
}

func (s *Server) dacs(ctx context.Context, env models.Environment) (map[string][]models.DedicatedAICluster, []string, error) {
	grouped, warnings, err := query.DACs(ctx, s.source(), env)
	s.logLoadWarnings("dedicated AI clusters", warnings)
	return grouped, warnings, err
}

func (s *Server) inferenceServices(ctx context.Context, env models.Environment) (map[string][]models.InferenceService, []string, error) {
	grouped, warnings, err := query.InferenceServices(ctx, s.source(), env)
	s.logLoadWarnings("inference services", warnings)
	return grouped, warnings, err
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
