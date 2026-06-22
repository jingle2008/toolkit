package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
)

// handleRepoWatchStarted records the live working-tree watch and arms the
// listener. Session-scoped: not gen-gated.
func (m *Model) handleRepoWatchStarted(msg repoWatchStartedMsg) tea.Cmd {
	m.repoWatching = true
	m.repoTrigger = msg.Trigger
	m.logger.Infow("repo watch started")
	return waitForRepoTriggerCmd(msg.Trigger)
}

// handleRepoWatchTriggered issues quiet background reloads (dataset, plus GPU
// pools when loaded) and re-arms the listener. No beginTask: a working-tree
// change must not flash the loading spinner.
func (m *Model) handleRepoWatchTriggered() tea.Cmd {
	m.logger.Debugw("repo watch triggered; reloading dataset")
	cmds := []tea.Cmd{
		reloadDatasetCmd(m.sessionCtx(), m.loader, m.repoPath, m.environment, m.logger),
	}
	if m.dataset != nil && m.dataset.GPUPools != nil {
		cmds = append(cmds, reloadGPUPoolsCmd(m.sessionCtx(), m.loader, m.repoPath, m.environment, m.logger))
	}
	if m.repoTrigger != nil {
		cmds = append(cmds, waitForRepoTriggerCmd(m.repoTrigger))
	}
	return tea.Batch(cmds...)
}

// handleRepoWatchClosed clears the live repo indicator. No auto-reconnect;
// the watch is re-established only by an explicit manual refresh (see
// maybeStartRepoWatchCmd).
func (m *Model) handleRepoWatchClosed() {
	m.repoWatching = false
	m.logger.Warnw("repo watch closed; live repo indicator dropped")
}

// maybeStartRepoWatchCmd returns a command to (re)establish the repo
// working-tree watch when it is not currently active, else nil. The repo watch
// is session-scoped and started once in Init; if it drops (stream error) there
// is no auto-reconnect, so manual refresh uses this to recover it. The
// !repoWatching guard prevents starting a second watcher when one is already
// live (a redundant start during the brief Init→started window is harmless:
// both watchers are parented on the session context and stop at shutdown).
func (m *Model) maybeStartRepoWatchCmd() tea.Cmd {
	if m.repoWatching {
		return nil
	}
	return startRepoWatchCmd(m.sessionCtx(), m.loader, m.repoPath)
}

// handleDatasetReloaded merges the freshly loaded repo-owned data into the
// in-memory dataset (preserving live k8s fields). When the on-screen category
// is repo-backed it refreshes the view, preserving the active filter and
// selected-row cursor; when a k8s-backed category is showing, the merge cannot
// have changed its visible rows, so the recompute is skipped (the merged data
// is still cached for when the user navigates to a repo category).
func (m *Model) handleDatasetReloaded(msg datasetReloadedMsg) {
	if msg.Dataset == nil {
		return
	}
	if m.dataset == nil {
		m.dataset = msg.Dataset
	} else {
		m.dataset.MergeReloadedRepoData(msg.Dataset)
	}
	if !m.category.NeedsKubeConfig() {
		m.refreshDisplay()
	}
}

// handleGPUPoolsReloaded refreshes the cached GPU pool list and re-runs the
// pool enrichment (task-neutral). The view is rebuilt only when GPU pools are
// on screen.
func (m *Model) handleGPUPoolsReloaded(msg gpuPoolsReloadedMsg) tea.Cmd {
	if m.dataset == nil {
		return nil
	}
	m.dataset.GPUPools = msg.Items
	if m.category == domain.GPUPool {
		m.refreshDisplay()
	}
	return m.updateGPUPoolState()
}
