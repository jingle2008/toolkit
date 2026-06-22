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

// startK8sWatchCmd type-asserts the loader to loader.Watcher and starts the
// watch for cat. On success it returns k8sWatchStartedMsg with the trigger
// channel; if the loader doesn't support watching or setup fails, it
// returns k8sWatchUnavailableMsg so the caller keeps the one-shot load
// result with no live indicator.
func startK8sWatchCmd(ctx context.Context, ld loader.Composite, cat domain.Category, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		w, ok := ld.(loader.Watcher)
		if !ok {
			return k8sWatchUnavailableMsg{Cat: cat, Gen: gen}
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
			return k8sWatchUnavailableMsg{Cat: cat, Gen: gen}
		}
		if err != nil {
			return k8sWatchUnavailableMsg{Cat: cat, Gen: gen}
		}
		return k8sWatchStartedMsg{Cat: cat, Trigger: trigger, Gen: gen}
	}
}

// waitForK8sTriggerCmd blocks (in the tea runtime's goroutine) on one value
// from the trigger channel: a tick → k8sWatchTriggeredMsg, a close →
// k8sWatchClosedMsg.
func waitForK8sTriggerCmd(cat domain.Category, trigger <-chan struct{}, gen int) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-trigger; !ok {
			return k8sWatchClosedMsg{Cat: cat, Gen: gen}
		}
		return k8sWatchTriggeredMsg{Cat: cat, Gen: gen}
	}
}

// startRepoWatchCmd type-asserts the loader to loader.RepoWatcher and starts
// the working-tree watch on the session context. On success it returns
// repoWatchStartedMsg; if the loader doesn't support watching or setup fails,
// it returns repoWatchClosedMsg so the app runs static with no live indicator.
func startRepoWatchCmd(ctx context.Context, ld loader.Composite, repoPath string) tea.Cmd {
	return func() tea.Msg {
		rw, ok := ld.(loader.RepoWatcher)
		if !ok {
			return repoWatchClosedMsg{}
		}
		trigger, err := rw.WatchRepo(ctx, repoPath)
		if err != nil {
			return repoWatchClosedMsg{}
		}
		return repoWatchStartedMsg{Trigger: trigger}
	}
}

// waitForRepoTriggerCmd blocks on one value from the repo trigger: a tick →
// repoWatchTriggeredMsg, a close → repoWatchClosedMsg.
func waitForRepoTriggerCmd(trigger <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-trigger; !ok {
			return repoWatchClosedMsg{}
		}
		return repoWatchTriggeredMsg{}
	}
}

// reloadDatasetCmd re-runs LoadDataset on the session context and returns
// datasetReloadedMsg. On error it logs at warn and returns nil — a background
// refresh must not raise a toast or disturb the loading state.
func reloadDatasetCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger logging.Logger) tea.Cmd {
	return func() tea.Msg {
		ds, err := ld.LoadDataset(ctx, repoPath, env)
		if err != nil {
			logger.Warnw("background dataset reload failed; keeping current data", "error", err)
			return nil
		}
		return datasetReloadedMsg{Dataset: ds}
	}
}

// reloadGPUPoolsCmd re-runs LoadGPUPools on the session context and returns
// gpuPoolsReloadedMsg. Like reloadDatasetCmd it is quiet on error.
func reloadGPUPoolsCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger logging.Logger) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUPools(ctx, repoPath, env)
		if err != nil {
			logger.Warnw("background GPU pool reload failed; keeping current data", "error", err)
			return nil
		}
		return gpuPoolsReloadedMsg{Items: items}
	}
}
