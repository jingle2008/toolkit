package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
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
	return func() tea.Msg {
		// DAC deletion is a multi-minute workflow with its own internal
		// timeout; use longOpCtx so the 30s one-shot cap doesn't cancel it
		// after endpoint deletion but before the cluster delete completes.
		ctx := m.longOpCtx()
		if err := actions.DeleteDedicatedAICluster(ctx, dac, m.environment, m.logger); err != nil {
			return deleteErrMsg{
				err:       err,
				category:  domain.DedicatedAICluster,
				key:       itemKey,
				prevState: prevState,
			}
		}
		return deleteDoneMsg{
			category: domain.DedicatedAICluster,
			key:      itemKey,
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
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		if err := actions.TerminateInstance(ctx, node, m.environment, m.logger); err != nil {
			return deleteErrMsg{
				err:      err,
				category: domain.GPUNode,
				key:      itemKey,
			}
		}
		return deleteDoneMsg{
			category: domain.GPUNode,
			key:      itemKey,
		}
	}
}

func (m *Model) rebootNode(item any, itemKey models.ItemKey) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for reboot operation", "category", m.category)
		return nil
	}

	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for reboot operation", "item", item)
		return nil
	}

	if node.GetStatus() == "Rebooting" {
		return nil
	}
	// optimistic UI
	node.SetStatus("Rebooting")
	m.updateRows(false)

	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := actions.SoftResetInstance(ctx, node, m.environment, m.logger)
		return rebootNodeResultMsg{key: itemKey, err: err}
	}
}

func (m *Model) handleDeleteErrMsg(msg deleteErrMsg) {
	m.logger.Errorw("failed to delete item", "key", msg.key, "error", msg.err)
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
}

func (m *Model) handleDeleteDoneMsg(msg deleteDoneMsg) {
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

func (m *Model) handleUpdateDoneMsg(msg updateDoneMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to update data", "category", msg.category, "error", msg.err)
		for i := range m.dataset.GPUPools {
			m.dataset.GPUPools[i].Status = "UNKNOWN"
		}
	}

	// update view if current
	if msg.category == m.category {
		m.updateRows(false)
	}
}

func (m *Model) handleGPUPoolScaleStartedMsg(msg gpuPoolScaleStartedMsg) {
	item := findItem(m.dataset, domain.GPUPool, msg.key)
	if pool, ok := item.(*models.GPUPool); ok && pool != nil {
		pool.Status = "SCALING"
		if m.category == domain.GPUPool {
			m.updateRows(false)
		}
	}
}

func (m *Model) handleGPUPoolScaleResultMsg(msg gpuPoolScaleResultMsg) {
	item := findItem(m.dataset, domain.GPUPool, msg.key)
	if pool, ok := item.(*models.GPUPool); ok && pool != nil {
		if msg.err != nil {
			m.logger.Errorw("failed to scale GPU pool", "key", msg.key, "error", msg.err)
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
	item := findItem(m.dataset, domain.GPUNode, msg.key)
	if node, ok := item.(*models.GPUNode); ok && node != nil {
		if msg.err != nil {
			m.logger.Errorw("failed to toggle cordon state", "key", msg.key, "error", msg.err)
		} else {
			node.IsSchedulingDisabled = msg.state
			// Clear transient status so GetStatus reflects current state
			node.SetStatus("")
		}
		if m.category == domain.GPUNode {
			m.updateRows(false)
		}
	}
}

func (m *Model) handleDrainNodeResultMsg(msg drainNodeResultMsg) {
	if msg.err != nil {
		m.logger.Errorw("failed to drain node", "key", msg.key, "error", msg.err)
	}
	if m.category == domain.GPUNode {
		m.updateRows(false)
	}
}

func (m *Model) handleRebootNodeResultMsg(msg rebootNodeResultMsg) {
	item := findItem(m.dataset, domain.GPUNode, msg.key)
	if node, ok := item.(*models.GPUNode); ok && node != nil {
		if msg.err != nil {
			m.logger.Errorw("failed to reboot node", "key", msg.key, "error", msg.err)
			node.SetStatus("FAILED")
		} else {
			// Clear transient "Rebooting" status to recompute via GetStatus()
			node.SetStatus("")
		}
	}
	if m.category == domain.GPUNode {
		m.updateRows(false)
	}
}
