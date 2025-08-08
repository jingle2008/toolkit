package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
)

type loadRequest struct {
	category domain.Category
	model    *Model
}

func (r loadRequest) Run() tea.Msg {
	var (
		data any
		err  error
	)
	switch r.category { //nolint:exhaustive
	case domain.BaseModel:
		data, err = r.model.loader.LoadBaseModels(r.model.ctx, r.model.kubeConfig, r.model.environment)
	case domain.GpuPool:
		data, err = r.model.loader.LoadGpuPools(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.GpuNode:
		data, err = r.model.loader.LoadGpuNodes(r.model.ctx, r.model.kubeConfig, r.model.environment)
	case domain.DedicatedAICluster:
		data, err = r.model.loader.LoadDedicatedAIClusters(r.model.ctx, r.model.kubeConfig, r.model.environment)
	case domain.Tenant, domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		data, err = r.model.loader.LoadTenancyOverrideGroup(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.LimitRegionalOverride:
		data, err = r.model.loader.LoadLimitRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.ConsolePropertyRegionalOverride:
		data, err = r.model.loader.LoadConsolePropertyRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.PropertyRegionalOverride:
		data, err = r.model.loader.LoadPropertyRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	}
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to load %s: %w", r.category, err))
	}
	return DataMsg{Data: data}
}
