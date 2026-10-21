package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the restart patch, the cordon and the scale-up.
var (
	restartInferenceServiceFn = k8s.RestartInferenceService
	setCordonFn               = k8s.SetCordon
	increasePoolSizeFn        = oci.IncreasePoolSize
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
		m.logger.Errorw("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	env := m.environment
	return m.trackAction("delete "+dac.Name, "Deleting", itemKey, func(progress func(string)) tea.Msg {
		// DAC deletion is a multi-minute workflow with its own internal
		// timeout; use longOpCtx so the 30s one-shot cap doesn't cancel it
		// after endpoint deletion but before the cluster delete completes.
		ctx := m.longOpCtx()
		logger := progressLogger{Logger: m.logger, report: progress}
		err := audited(m.logger, mutation.DeleteDAC, dac.Name, func() error {
			return oci.DeleteDedicatedAICluster(ctx, dac, env, logger)
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
		m.logger.Errorw("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	env := m.environment
	return m.trackAction("terminate "+node.Name, "Terminating", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.TerminateNode, node.Name, func() error {
			return oci.TerminateInstance(ctx, node, env, m.logger)
		})
		if err != nil {
			return deleteErrMsg{err: err, category: domain.GPUNode, key: itemKey, env: env}
		}
		return deleteDoneMsg{category: domain.GPUNode, key: itemKey, env: env}
	})
}

func (m *Model) restartNode(item any, itemKey models.ItemKey) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for restart operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for restart operation", "item", item)
		return nil
	}
	env := m.environment
	return m.trackAction("restart "+node.Name, "Restarting", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.RestartNode, node.Name, func() error {
			return oci.SoftResetInstance(ctx, node, env, m.logger)
		})
		return restartNodeResultMsg{key: itemKey, err: err, env: env}
	})
}

// handleDeleteErrMsg says a delete failed, wherever it ran.
func (m *Model) handleDeleteErrMsg(msg deleteErrMsg) tea.Cmd {
	m.logger.Errorw("failed to delete item", "key", msg.key, "error", msg.err)
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
	removeItemFromDataset(m.dataset, msg.category, msg.key)
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
// SCALING) it schedules the next enrichment.
func (m *Model) handleUpdateDoneMsg(msg updateDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.logger.Errorw("failed to update data", "category", msg.category, "error", msg.err)
	}
	if m.dataset == nil || !m.inCurrentEnv(msg.env) {
		return nil
	}
	enriched := make(map[string]models.GPUPool, len(msg.pools))
	for _, p := range msg.pools {
		enriched[p.Name] = p
	}
	for i := range m.dataset.GPUPools {
		pool := &m.dataset.GPUPools[i]
		if msg.err != nil {
			pool.Status = models.GPUPoolStatusUnknown
			continue
		}
		if e, ok := enriched[pool.Name]; ok {
			pool.ID, pool.ActualSize, pool.Status = e.ID, e.ActualSize, e.Status
		}
	}

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
		m.logger.Errorw("failed to scale GPU pool", "key", msg.key, "error", msg.err)
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
		m.logger.Errorw("failed to toggle cordon state", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("%s %s failed: %v", verb, target, msg.err), keys.ToggleCordon.Help().Key+" retry")
	}
	if m.inCurrentEnv(msg.env) {
		if node, ok := findItem(m.dataset, domain.GPUNode, msg.key).(*models.GPUNode); ok && node != nil {
			node.IsSchedulingDisabled = msg.state
			if m.category == domain.GPUNode {
				m.updateRows(false)
			}
		}
	}
	return m.showToast(verb+"ed "+target, toastInfo)
}

func (m *Model) handleDrainNodeResultMsg(msg drainNodeResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Errorw("failed to drain node", "key", msg.key, "error", msg.err)
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
		m.logger.Errorw("unsupported item type for restart operation", "item", item)
		return nil
	}
	namespace, name := svc.Namespace, svc.Name
	env, kubeConfig := m.environment, m.kubeConfig
	return m.trackAction("restart "+namespace+"/"+name, "", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.RestartInferenceService, namespace+"/"+name, func() error {
			return restartInferenceServiceFn(ctx, kubeConfig, env.KubeContext(), namespace, name)
		})
		return restartServiceResultMsg{key: itemKey, err: err, env: env}
	})
}

func (m *Model) handleRestartServiceResultMsg(msg restartServiceResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Errorw("failed to restart inference service", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("restart %s failed: %v", target, msg.err), keys.Restart.Help().Key+" retry")
	}
	return m.showToast("restart requested for "+target, toastInfo)
}

func (m *Model) handleRestartNodeResultMsg(msg restartNodeResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Errorw("failed to restart node", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("restart %s failed: %v", target, msg.err), keys.Restart.Help().Key+" retry")
	}
	return m.showToast("restart requested for "+target, toastInfo)
}
