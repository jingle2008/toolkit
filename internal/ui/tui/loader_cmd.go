package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func loadTenancyOverrideGroupCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		group, err := ld.LoadTenancyOverrideGroup(ctx, repoPath, env)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", domain.Tenant, err), Gen: gen, Cat: domain.Tenant}
		}
		return tenancyOverridesLoadedMsg{Group: group, Gen: gen}
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
		src := lazySources[cat]
		if src.watch == nil {
			return k8sWatchUnavailableMsg{Cat: cat, Gen: gen}
		}
		trigger, err := src.watch(ctx, w, kubeCfg, env)
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
// datasetReloadedMsg. On error it logs at warn and reports the error in the
// message, keeping the current data: a background refresh must not disturb
// the loading state, but a repo edit that broke the load (a malformed
// file) must not go unnoticed under LIVE either.
func reloadDatasetCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger *slog.Logger) tea.Cmd {
	return func() tea.Msg {
		ds, err := ld.LoadDataset(ctx, repoPath, env)
		if err != nil {
			logger.Warn("background dataset reload failed; keeping current data", "error", err)
			return datasetReloadedMsg{env: env, err: err}
		}
		return datasetReloadedMsg{Dataset: ds, env: env}
	}
}

// reloadGPUPoolsCmd re-runs LoadGPUPools on the session context and returns
// gpuPoolsReloadedMsg. Like reloadDatasetCmd it is quiet on error.
func reloadGPUPoolsCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger *slog.Logger) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUPools(ctx, repoPath, env)
		if err != nil {
			logger.Warn("background GPU pool reload failed; keeping current data", "error", err)
			return nil
		}
		return gpuPoolsReloadedMsg{Items: items, env: env}
	}
}

// kubeVersionUnknown is the version shown when the fetch failed.
const kubeVersionUnknown = "n/a"

// fetchKubeVersionCmd queries the API server version for kubeCtx. Failures
// render as "n/a" rather than a toast — the header is informational only.
// A context the kubeconfig lacks is reported as NoContext.
func fetchKubeVersionCmd(kubeConfig, kubeCtx string) tea.Cmd {
	return func() tea.Msg {
		if ok, err := k8s.HasContext(kubeConfig, kubeCtx); err == nil && !ok {
			return kubeVersionMsg{Context: kubeCtx, Version: kubeVersionUnknown, NoContext: true}
		}
		v, err := k8s.ServerVersion(kubeConfig, kubeCtx)
		if err != nil || v == "" {
			v = kubeVersionUnknown
		}
		return kubeVersionMsg{Context: kubeCtx, Version: v}
	}
}
