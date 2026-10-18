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

// Seams tests use to fake the restart patch and the cordon.
var (
	restartInferenceServiceFn = k8s.RestartInferenceService
	setCordonFn               = k8s.SetCordon
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
deleteDedicatedAICluster deletes a DedicatedAICluster item and updates the UI accordingly.
*/
func (m *Model) deleteDedicatedAICluster(itemKey models.ItemKey) tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	dac, ok := item.(*models.DedicatedAICluster)
	if !ok || dac == nil {
		m.logger.Errorw("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	if dac.Status == "Deleting" {
		return nil
	}
	prevState := dac.Status
	dac.Status = "Deleting"
	m.updateRows(false)
	env := m.environment
	return func() tea.Msg {
		// DAC deletion is a multi-minute workflow with its own internal
		// timeout; use longOpCtx so the 30s one-shot cap doesn't cancel it
		// after endpoint deletion but before the cluster delete completes.
		ctx := m.longOpCtx()
		err := audited(m.logger, mutation.DeleteDAC, dac.Name, func() error {
			return oci.DeleteDedicatedAICluster(ctx, dac, env, m.logger)
		})
		if err != nil {
			return deleteErrMsg{
				err:       err,
				category:  domain.DedicatedAICluster,
				key:       itemKey,
				prevState: prevState,
				env:       env,
			}
		}
		return deleteDoneMsg{
			category: domain.DedicatedAICluster,
			key:      itemKey,
			env:      env,
		}
	}
}

func (m *Model) deleteGPUNode(itemKey models.ItemKey) tea.Cmd {
	item := findItem(m.dataset, m.category, itemKey)
	node, ok := item.(*models.GPUNode)
	if !ok || node == nil {
		m.logger.Errorw("item not found for delete operation", "category", m.category, "key", itemKey)
		return nil
	}
	if node.GetStatus() == "Deleting" {
		return nil
	}
	node.SetStatus("Deleting")
	m.updateRows(false)
	env := m.environment
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.TerminateNode, node.Name, func() error {
			return oci.TerminateInstance(ctx, node, env, m.logger)
		})
		if err != nil {
			return deleteErrMsg{
				err:      err,
				category: domain.GPUNode,
				key:      itemKey,
				env:      env,
			}
		}
		return deleteDoneMsg{
			category: domain.GPUNode,
			key:      itemKey,
			env:      env,
		}
	}
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

	if node.GetStatus() == "Restarting" {
		return nil
	}
	// optimistic UI
	node.SetStatus("Restarting")
	m.updateRows(false)

	env := m.environment
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.RestartNode, node.Name, func() error {
			return oci.SoftResetInstance(ctx, node, env, m.logger)
		})
		return restartNodeResultMsg{key: itemKey, err: err, env: env}
	}
}

// handleDeleteErrMsg restores the row a failed delete marked and says it
// failed — it used to only log, so the failure went unseen.
func (m *Model) handleDeleteErrMsg(msg deleteErrMsg) tea.Cmd {
	m.logger.Errorw("failed to delete item", "key", msg.key, "error", msg.err)
	if !m.inCurrentEnv(msg.env) {
		return nil
	}
	item := findItem(m.dataset, msg.category, msg.key)

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		dac.Status = msg.prevState
	} else if node, ok := item.(*models.GPUNode); ok {
		node.SetStatus(msg.prevState)
	}

	// update view if current
	if msg.category == m.category {
		m.updateRows(false)
	}
	return m.showErrorWithNext(fmt.Sprintf("delete %s failed: %v", itemKeyString(msg.key), msg.err),
		keys.Delete.Help().Key+" retry")
}

func (m *Model) handleDeleteDoneMsg(msg deleteDoneMsg) {
	if !m.inCurrentEnv(msg.env) {
		return
	}
	removeItemFromDataset(m.dataset, msg.category, msg.key)

	// update view if current
	if msg.category == m.category {
		idx := m.table.Cursor()
		if idx+1 >= len(m.table.Rows()) {
			m.table.MoveUp(1)
		}
		m.updateRows(false)
	}
}

// handleUpdateDoneMsg applies a GPU pool enrichment (updateGPUPoolState):
// each current pool takes the enriched ID, actual size and status of the
// snapshot pool of its name. A pool added since the snapshot keeps its
// state until the next enrichment.
func (m *Model) handleUpdateDoneMsg(msg updateDoneMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to update data", "category", msg.category, "error", msg.err)
	}
	if m.dataset == nil || !m.inCurrentEnv(msg.env) {
		return
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
}

func (m *Model) handleGPUPoolScaleStartedMsg(msg gpuPoolScaleStartedMsg) {
	if !m.inCurrentEnv(msg.env) {
		return
	}
	item := findItem(m.dataset, domain.GPUPool, msg.key)
	if pool, ok := item.(*models.GPUPool); ok && pool != nil {
		pool.Status = "SCALING"
		if m.category == domain.GPUPool {
			m.updateRows(false)
		}
	}
}

func (m *Model) handleGPUPoolScaleResultMsg(msg gpuPoolScaleResultMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to scale GPU pool", "key", msg.key, "error", msg.err)
	}
	if !m.inCurrentEnv(msg.env) {
		return
	}
	item := findItem(m.dataset, domain.GPUPool, msg.key)
	if pool, ok := item.(*models.GPUPool); ok && pool != nil {
		if msg.err != nil {
			pool.Status = "FAILED"
		} else {
			// Optimistic success state until enrichment updates the pool
			pool.Status = "RUNNING"
		}
		if m.category == domain.GPUPool {
			m.updateRows(false)
		}
	}
}

func (m *Model) handleCordonNodeResultMsg(msg cordonNodeResultMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to toggle cordon state", "key", msg.key, "error", msg.err)
	}
	if !m.inCurrentEnv(msg.env) {
		return
	}
	item := findItem(m.dataset, domain.GPUNode, msg.key)
	if node, ok := item.(*models.GPUNode); ok && node != nil {
		if msg.err == nil {
			node.IsSchedulingDisabled = msg.state
			// Clear transient status so GetStatus reflects current state
			node.SetStatus("")
		}
		if m.category == domain.GPUNode {
			m.updateRows(false)
		}
	}
}

func (m *Model) handleDrainNodeResultMsg(msg drainNodeResultMsg) tea.Cmd {
	if m.category == domain.GPUNode {
		m.updateRows(false)
	}
	target := itemKeyString(msg.key)
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
	kubeConfig, kubeContext := m.kubeConfig, m.environment.KubeContext()
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.RestartInferenceService, namespace+"/"+name, func() error {
			return restartInferenceServiceFn(ctx, kubeConfig, kubeContext, namespace, name)
		})
		return restartServiceResultMsg{key: itemKey, err: err}
	}
}

func (m *Model) handleRestartServiceResultMsg(msg restartServiceResultMsg) tea.Cmd {
	target := itemKeyString(msg.key)
	if msg.err != nil {
		m.logger.Errorw("failed to restart inference service", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("restart %s failed: %v", target, msg.err), keys.Restart.Help().Key+" retry")
	}
	return m.showToast("restart requested for "+target, toastInfo)
}

func (m *Model) handleRestartNodeResultMsg(msg restartNodeResultMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to restart node", "key", msg.key, "error", msg.err)
	}
	if !m.inCurrentEnv(msg.env) {
		return
	}
	item := findItem(m.dataset, domain.GPUNode, msg.key)
	if node, ok := item.(*models.GPUNode); ok && node != nil {
		if msg.err != nil {
			node.SetStatus("FAILED")
		} else {
			// Clear transient "Restarting" status to recompute via GetStatus()
			node.SetStatus("")
		}
	}
	if m.category == domain.GPUNode {
		m.updateRows(false)
	}
}
