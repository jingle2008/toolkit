package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

type loadRequest struct {
	category    domain.Category
	loader      loader.Loader
	ctx         context.Context
	repoPath    string
	kubeConfig  string
	environment models.Environment
	gen         int
}

func (r loadRequest) Run() tea.Msg {
	var (
		data any
		err  error
	)
	switch r.category { //nolint:exhaustive
	case domain.BaseModel:
		data, err = r.loader.LoadBaseModels(r.ctx, r.kubeConfig, r.environment)
	case domain.GpuPool:
		data, err = r.loader.LoadGpuPools(r.ctx, r.repoPath, r.environment)
	case domain.GpuNode:
		data, err = r.loader.LoadGpuNodes(r.ctx, r.kubeConfig, r.environment)
	case domain.DedicatedAICluster:
		data, err = r.loader.LoadDedicatedAIClusters(r.ctx, r.kubeConfig, r.environment)
	case domain.Tenant, domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		data, err = r.loader.LoadTenancyOverrideGroup(r.ctx, r.repoPath, r.environment)
	case domain.LimitRegionalOverride:
		data, err = r.loader.LoadLimitRegionalOverrides(r.ctx, r.repoPath, r.environment)
	case domain.ConsolePropertyRegionalOverride:
		data, err = r.loader.LoadConsolePropertyRegionalOverrides(r.ctx, r.repoPath, r.environment)
	case domain.PropertyRegionalOverride:
		data, err = r.loader.LoadPropertyRegionalOverrides(r.ctx, r.repoPath, r.environment)
	}
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to load %s: %w", r.category, err))
	}
	switch r.category { //nolint:exhaustive
	case domain.BaseModel:
		return baseModelsLoadedMsg{Items: data.([]models.BaseModel), Gen: r.gen}
	case domain.GpuPool:
		return gpuPoolsLoadedMsg{Items: data.([]models.GpuPool), Gen: r.gen}
	case domain.GpuNode:
		return gpuNodesLoadedMsg{Items: data.(map[string][]models.GpuNode), Gen: r.gen}
	case domain.DedicatedAICluster:
		return dedicatedAIClustersLoadedMsg{Items: data.(map[string][]models.DedicatedAICluster), Gen: r.gen}
	case domain.Tenant, domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		return tenancyOverridesLoadedMsg{Group: data.(models.TenancyOverrideGroup), Gen: r.gen}
	case domain.LimitRegionalOverride:
		return limitRegionalOverridesLoadedMsg{Items: data.([]models.LimitRegionalOverride), Gen: r.gen}
	case domain.ConsolePropertyRegionalOverride:
		return consolePropertyRegionalOverridesLoadedMsg{Items: data.([]models.ConsolePropertyRegionalOverride), Gen: r.gen}
	case domain.PropertyRegionalOverride:
		return propertyRegionalOverridesLoadedMsg{Items: data.([]models.PropertyRegionalOverride), Gen: r.gen}
	default:
		// Fallback to generic message if a new category is added without a typed msg
		return DataMsg{Data: data, Gen: r.gen}
	}
}

// Pure command constructors (preferred over loadRequest)
func loadBaseModelsCmd(ld loader.Loader, ctx context.Context, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadBaseModels(ctx, kubeCfg, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.BaseModel, err))
		}
		return baseModelsLoadedMsg{Items: items, Gen: gen}
	}
}

func loadGpuPoolsCmd(ld loader.Loader, ctx context.Context, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGpuPools(ctx, repoPath, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.GpuPool, err))
		}
		return gpuPoolsLoadedMsg{Items: items, Gen: gen}
	}
}

func loadGpuNodesCmd(ld loader.Loader, ctx context.Context, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGpuNodes(ctx, kubeCfg, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.GpuNode, err))
		}
		return gpuNodesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadDedicatedAIClustersCmd(ld loader.Loader, ctx context.Context, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadDedicatedAIClusters(ctx, kubeCfg, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.DedicatedAICluster, err))
		}
		return dedicatedAIClustersLoadedMsg{Items: items, Gen: gen}
	}
}

func loadTenancyOverrideGroupCmd(ld loader.Loader, ctx context.Context, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		group, err := ld.LoadTenancyOverrideGroup(ctx, repoPath, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.Tenant, err))
		}
		return tenancyOverridesLoadedMsg{Group: group, Gen: gen}
	}
}

func loadLimitRegionalOverridesCmd(ld loader.Loader, ctx context.Context, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadLimitRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.LimitRegionalOverride, err))
		}
		return limitRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadConsolePropertyRegionalOverridesCmd(ld loader.Loader, ctx context.Context, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadConsolePropertyRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.ConsolePropertyRegionalOverride, err))
		}
		return consolePropertyRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadPropertyRegionalOverridesCmd(ld loader.Loader, ctx context.Context, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadPropertyRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to load %s: %w", domain.PropertyRegionalOverride, err))
		}
		return propertyRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}
