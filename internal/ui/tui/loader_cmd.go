package tui

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Pure command constructors. Each builds a tea.Cmd that loads one
// category and returns a typed *LoadedMsg on success or errMsg on
// failure; the gen counter lets the reducer drop stale responses.
func loadBaseModelsCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadBaseModels(ctx, kubeCfg, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.BaseModel, err), Gen: gen}
		}
		return baseModelsLoadedMsg{Items: items, Gen: gen}
	}
}

func loadImportedModelsCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		grouped, err := ld.LoadImportedModels(ctx, kubeCfg, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.ImportedModel, err), Gen: gen}
		}
		return importedModelsLoadedMsg{Items: grouped, Gen: gen}
	}
}

func loadGPUPoolsCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUPools(ctx, repoPath, env)
		if err != nil {
			// Partial-success is non-fatal in the TUI: items still has
			// the rows we could load, and the per-source error has
			// already been logged inside the terraform package.
			if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
				logging.FromContext(ctx).Warnw("loaded GPU pools with partial failures",
					"category", domain.GPUPool, "error", partial)
				return gpuPoolsLoadedMsg{Items: items, Gen: gen}
			}
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.GPUPool, err), Gen: gen}
		}
		return gpuPoolsLoadedMsg{Items: items, Gen: gen}
	}
}

func loadGPUNodesCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUNodesByPool(ctx, kubeCfg, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.GPUNode, err), Gen: gen}
		}
		return gpuNodesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadGPUWorkloadsCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUWorkloadsByNode(ctx, kubeCfg, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.GPUWorkload, err), Gen: gen}
		}
		return gpuWorkloadsLoadedMsg{Items: items, Gen: gen}
	}
}

func loadDedicatedAIClustersCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadDedicatedAIClusters(ctx, kubeCfg, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.DedicatedAICluster, err), Gen: gen}
		}
		return dedicatedAIClustersLoadedMsg{Items: items, Gen: gen}
	}
}

func loadTenancyOverrideGroupCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		group, err := ld.LoadTenancyOverrideGroup(ctx, repoPath, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.Tenant, err), Gen: gen}
		}
		return tenancyOverridesLoadedMsg{Group: group, Gen: gen}
	}
}

func loadLimitRegionalOverridesCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadLimitRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.LimitRegionalOverride, err), Gen: gen}
		}
		return limitRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadConsolePropertyRegionalOverridesCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadConsolePropertyRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.ConsolePropertyRegionalOverride, err), Gen: gen}
		}
		return consolePropertyRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}

func loadPropertyRegionalOverridesCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadPropertyRegionalOverrides(ctx, repoPath, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.PropertyRegionalOverride, err), Gen: gen}
		}
		return propertyRegionalOverridesLoadedMsg{Items: items, Gen: gen}
	}
}

// startWatchCmd type-asserts the loader to loader.Watcher and starts the
// watch for cat. On success it returns watchStartedMsg with the trigger
// channel; if the loader doesn't support watching or setup fails, it
// returns watchUnavailableMsg so the caller keeps the one-shot load
// result with no live indicator.
func startWatchCmd(ctx context.Context, ld loader.Composite, cat domain.Category, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		w, ok := ld.(loader.Watcher)
		if !ok {
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		var (
			trigger <-chan struct{}
			err     error
		)
		switch cat {
		case domain.BaseModel:
			trigger, err = w.WatchBaseModels(ctx, kubeCfg, env)
		case domain.ImportedModel:
			trigger, err = w.WatchImportedModels(ctx, kubeCfg, env)
		case domain.GPUNode:
			trigger, err = w.WatchGPUNodes(ctx, kubeCfg, env)
		case domain.GPUWorkload:
			trigger, err = w.WatchGPUWorkloads(ctx, kubeCfg, env)
		case domain.DedicatedAICluster:
			trigger, err = w.WatchDedicatedAIClusters(ctx, kubeCfg, env)
		default:
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		if err != nil {
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		return watchStartedMsg{Cat: cat, Trigger: trigger, Gen: gen}
	}
}

// waitForTriggerCmd blocks (in the tea runtime's goroutine) on one value
// from the trigger channel: a tick → watchTriggeredMsg, a close →
// watchClosedMsg.
func waitForTriggerCmd(cat domain.Category, trigger <-chan struct{}, gen int) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-trigger; !ok {
			return watchClosedMsg{Cat: cat, Gen: gen}
		}
		return watchTriggeredMsg{Cat: cat, Gen: gen}
	}
}
