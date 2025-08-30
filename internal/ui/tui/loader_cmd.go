package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

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
