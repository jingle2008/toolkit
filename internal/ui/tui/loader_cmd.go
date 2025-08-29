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
	return DataMsg{Data: data}
}
