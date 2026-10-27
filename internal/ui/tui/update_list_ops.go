package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/mutation"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
deleteItem handles the generic delete action based on the current category.
For DedicatedAICluster, it deletes via SDK and removes the row locally.
*/
func (m *Model) deleteItem(itemKey models.ItemKey) tea.Cmd {
	switch m.category {
	case domain.DedicatedAICluster:
		return m.deleteDedicatedAICluster(itemKey)
	case domain.GPUNode:
		return m.deleteGPUNode(itemKey)
	case domain.InferenceService:
		return m.deleteOrphanInferenceService(itemKey, mutation.DeleteInferenceService)
	default:
		// exhaustive
	}
	return nil
}

/*
deleteDedicatedAICluster deletes a DedicatedAICluster: its endpoints, then
the cluster, a multi-minute OCI workflow whose steps show as progress.
*/
func (m *Model) deleteDedicatedAICluster(itemKey models.ItemKey) tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	dac, ok := item.(*models.DedicatedAICluster)
	if !ok || dac == nil {
		m.logger.Error("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	// DAC deletion is a multi-minute workflow with its own internal
	// timeout; use longOpCtx so the 30s one-shot cap doesn't cancel it
	// after endpoint deletion but before the cluster delete completes.
	// The action reads only what is captured here, never m.
	env, ctx, log, snapshot, deps := m.environment, m.longOpCtx(), m.logger, *dac, m.deps
	return m.trackAction("delete "+dac.Name, "Deleting", itemKey, func(progress func(string)) tea.Msg {
		logger := newProgressLogger(log, progress)
		err := audited(log, mutation.DeleteDAC, snapshot.Name, func() error {
			return deps.DeleteDAC(ctx, &snapshot, env, logger)
		})
		if err != nil {
			return deleteErrMsg{err: err, category: domain.DedicatedAICluster, key: itemKey, env: env}
		}
		return deleteDoneMsg{category: domain.DedicatedAICluster, key: itemKey, env: env}
	})
}

func (m *Model) deleteGPUNode(itemKey models.ItemKey) tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	node, ok := item.(*models.GPUNode)
	if !ok || node == nil {
		m.logger.Error("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	env, opCtx, logger, snapshot, deps := m.environment, m.opCtxFunc(), m.logger, *node, m.deps
	return m.trackAction("terminate "+node.Name, "Terminating", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := opCtx()
		defer cancel()
		err := audited(logger, mutation.TerminateNode, snapshot.Name, func() error {
			return deps.Terminate(ctx, &snapshot, env, logger)
		})
		if err != nil {
			return deleteErrMsg{err: err, category: domain.GPUNode, key: itemKey, env: env}
		}
		return deleteDoneMsg{category: domain.GPUNode, key: itemKey, env: env}
	})
}

func (m *Model) restartNode(item any, itemKey models.ItemKey) tea.Cmd {
	if item == nil {
		m.logger.Error("no item selected for restart operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Error("unsupported item type for restart operation", "item", item)
		return nil
	}
	env, opCtx, logger, snapshot, deps := m.environment, m.opCtxFunc(), m.logger, *node, m.deps
	return m.trackAction("restart "+node.Name, "Restarting", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := opCtx()
		defer cancel()
		err := audited(logger, mutation.RestartNode, snapshot.Name, func() error {
			return deps.SoftReset(ctx, &snapshot, env, logger)
		})
		return restartNodeResultMsg{key: itemKey, err: err, env: env}
	})
}

// handleDeleteErrMsg says a delete failed, wherever it ran.
func (m *Model) handleDeleteErrMsg(msg deleteErrMsg) tea.Cmd {
	m.logger.Error("failed to delete item", "key", msg.key, "error", msg.err)
	return m.showErrorWithNext(fmt.Sprintf("%s %s failed%s: %v", deleteVerb(msg.category), itemKeyString(msg.key), m.envNote(msg.env), msg.err),
		keys.Delete.Help().Key+" retry")
}

// handleDeleteDoneMsg removes the deleted row, when it is in the current
// environment, and says the delete went through.
func (m *Model) handleDeleteDoneMsg(msg deleteDoneMsg) tea.Cmd {
	done := deleteVerb(msg.category) + " requested for " + itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.category == domain.DedicatedAICluster {
		done = "deleted " + itemKeyString(msg.key) + m.envNote(msg.env)
	}
	toast := m.showToast(done, toastInfo)
	if !m.inCurrentEnv(msg.env) {
		return toast
	}
	m.editDataset(func(ds *models.Dataset) { removeItemFromDataset(ds, msg.category, msg.key) })
	if msg.category == m.category {
		idx := m.table.Cursor()
		if idx+1 >= len(m.table.Rows()) {
			m.table.MoveUp(1)
		}
		m.updateRows(false)
	}
	return toast
}

// deleteVerb is how a toast names deleting a row of cat: a GPU node's
// instance is terminated.
func deleteVerb(cat domain.Category) string {
	if cat == domain.GPUNode {
		return "terminate"
	}
	return "delete"
}

// handleUpdateDoneMsg applies a GPU pool enrichment (updateGPUPoolState):
// each current pool takes the enriched ID, actual size and status of the
// snapshot pool of its name. A pool added since the snapshot keeps its
// state until the next enrichment. While a pool is mid-transition (e.g.
// SCALING) it schedules the next enrichment. The pool list is replaced,
// not written through: a tea.Cmd may still be reading the old one.
func (m *Model) handleUpdateDoneMsg(msg updateDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.logger.Error("failed to update data", "category", msg.category, "error", msg.err)
	}
	if m.dataset == nil || !m.inCurrentEnv(msg.env) {
		return nil
	}
	enriched := make(map[string]models.GPUPool, len(msg.pools))
	for _, p := range msg.pools {
		enriched[p.Name] = p
	}
	pools := slices.Clone(m.dataset.GPUPools)
	for i := range pools {
		pool := &pools[i]
		if msg.err != nil {
			pool.Status = models.GPUPoolStatusUnknown
			continue
		}
		if e, ok := enriched[pool.Name]; ok {
			pool.ID, pool.ActualSize, pool.Status = e.ID, e.ActualSize, e.Status
		}
	}
	m.editDataset(func(ds *models.Dataset) { ds.GPUPools = pools })

	// update view if current
	if msg.category == m.category {
		m.updateRows(false)
	}
	if msg.err != nil {
		return nil
	}
	return m.scheduleGPUPoolPoll()
}

// handleGPUPoolScaleResultMsg says what the scale-up requested — the
// sizes, and that terraform apply reverts one past the declared size —
// and then re-fetches the pools' OCI state so the new size and state show
// without a manual refresh.
func (m *Model) handleGPUPoolScaleResultMsg(msg gpuPoolScaleResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to scale GPU pool", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("scale up %s failed: %v", target, msg.err), keys.ScaleUp.Help().Key+" retry")
	}
	text := fmt.Sprintf("scale-up requested for %s: %s", target, msg.scale)
	if msg.scale.After > msg.declared {
		text += fmt.Sprintf("; past its Terraform size (%d) until the next terraform apply", msg.declared)
	}
	toast := m.showToast(text, toastInfo)
	if !m.inCurrentEnv(msg.env) {
		return toast
	}
	return tea.Batch(toast, m.updateGPUPoolState())
}

func (m *Model) handleCordonNodeResultMsg(msg cordonNodeResultMsg) tea.Cmd {
	verb := "cordon"
	if !msg.state {
		verb = "uncordon"
	}
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to toggle cordon state", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("%s %s failed: %v", verb, target, msg.err), keys.ToggleCordon.Help().Key+" retry")
	}
	// The node is replaced, not written through (updateItemInMap).
	if k, ok := msg.key.(models.ScopedItemKey); ok && m.dataset != nil && m.inCurrentEnv(msg.env) {
		nodes, found := updateItemInMap(m.dataset.GPUNodeMap, k, func(n *models.GPUNode) {
			n.IsSchedulingDisabled = msg.state
		})
		if found {
			m.editDataset(func(ds *models.Dataset) { ds.GPUNodeMap = nodes })
		}
		if found && m.category == domain.GPUNode {
			m.updateRows(false)
		}
	}
	return m.showToast(verb+"ed "+target, toastInfo)
}

func (m *Model) handleDrainNodeResultMsg(msg drainNodeResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to drain node", "key", msg.key, "error", msg.err)
		// The drain is recorded on the node, so a rerun resumes it.
		return m.showErrorWithNext(fmt.Sprintf("drain %s failed: %v", target, msg.err), keys.DrainNode.Help().Key+" retry (resumes)")
	}
	return m.showToast("drained "+target, toastInfo)
}

// restartInferenceService stamps the selected InferenceService's
// restartedAt annotation. Fire-and-forget: the k8s watch picks up the
// resulting rollout, so the result only needs a toast.
func (m *Model) restartInferenceService(item any, itemKey models.ItemKey) tea.Cmd {
	svc, ok := item.(*models.InferenceService)
	if !ok || svc == nil {
		m.logger.Error("unsupported item type for restart operation", "item", item)
		return nil
	}
	namespace, name := svc.Namespace, svc.Name
	env, kubeConfig, opCtx, logger, deps := m.environment, m.kubeConfig, m.opCtxFunc(), m.logger, m.deps
	return m.trackAction("restart "+namespace+"/"+name, "", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := opCtx()
		defer cancel()
		err := audited(logger, mutation.RestartInferenceService, namespace+"/"+name, func() error {
			return deps.RestartISVC(ctx, kubeConfig, env.KubeContext(), namespace, name)
		})
		return restartServiceResultMsg{key: itemKey, err: err, env: env}
	})
}

func (m *Model) handleRestartServiceResultMsg(msg restartServiceResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to restart inference service", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("restart %s failed: %v", target, msg.err), keys.Restart.Help().Key+" retry")
	}
	return m.showToast("restart requested for "+target, toastInfo)
}

func (m *Model) handleRestartNodeResultMsg(msg restartNodeResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to restart node", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("restart %s failed: %v", target, msg.err), keys.Restart.Help().Key+" retry")
	}
	return m.showToast("restart requested for "+target, toastInfo)
}
