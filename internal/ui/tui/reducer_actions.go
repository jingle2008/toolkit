package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
handleAdditionalKeys processes extra key events for the current category.
Refactored to reduce cyclomatic complexity by extracting item actions.
*/
func (m *Model) handleAdditionalKeys(msg tea.KeyMsg) tea.Cmd {
	idx := slices.IndexFunc(m.keys.Context, func(b key.Binding) bool {
		return key.Matches(msg, b)
	})

	if idx < 0 {
		return nil
	}

	binding := m.keys.Context[idx]
	if column, ok := strings.CutPrefix(binding.Help().Desc, keys.SortPrefix); ok {
		return m.sortTableByColumn(column)
	}

	if key.Matches(msg, keys.ToggleFaulty) {
		return m.toggleFaultyList()
	}

	return m.handleItemActions(msg)
}

// handleItemActions processes per-row actions for the current category.
func (m *Model) handleItemActions(msg tea.KeyMsg) tea.Cmd {
	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	item := findItem(m.dataset, m.category, itemKey)
	switch {
	case key.Matches(msg, keys.CopyTenant):
		return m.copyTenantID(item)
	case key.Matches(msg, keys.Refresh):
		return tea.Sequence(m.updateCategoryNoHist(m.category)...)
	case key.Matches(msg, keys.ToggleCordon):
		return m.cordonNode(item)
	case key.Matches(msg, keys.DrainNode):
		return m.drainNode(item)
	case key.Matches(msg, keys.Delete):
		return m.deleteItem(itemKey)
	case key.Matches(msg, keys.RebootNode):
		return m.rebootNode(item)
	case key.Matches(msg, keys.ScaleUp):
		return m.scaleUpGPUPool(item)
	}
	return nil
}

func (m *Model) copyTenantID(item any) tea.Cmd {
	return func() tea.Msg {
		actions.CopyTenantID(item, m.environment, m.logger)
		return nil
	}
}

func (m *Model) scaleUpGPUPool(item any) tea.Cmd {
	pool, ok := item.(*models.GPUPool)
	if !ok || pool == nil {
		m.logger.Errorw("no GPU pool selected for scale up")
		return nil
	}

	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	m.logger.Infow("action started", "action", "scaleUpGPUPool", "pool", itemKeyString(itemKey))
	return tea.Batch(
		func() tea.Msg { return gpuPoolScaleStartedMsg{key: itemKey} },
		func() tea.Msg {
			ctx, cancel := m.opCtx()
			defer cancel()
			err := actions.IncreasePoolSize(ctx, pool, m.environment, m.logger)
			return gpuPoolScaleResultMsg{key: itemKey, err: err}
		},
	)
}

func (m *Model) toggleFaultyList() tea.Cmd {
	m.showFaulty = !m.showFaulty
	return m.updateRowsAsync()
}

func (m *Model) cordonNode(item any) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for cordon operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for cordon operation", "item", item)
		return nil
	}
	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	m.logger.Infow("action started", "action", "toggleCordon", "node", itemKeyString(itemKey))
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		state, err := k8s.ToggleCordon(ctx, m.kubeConfig, m.environment.KubeContext(), node.Name)
		return cordonNodeResultMsg{key: itemKey, state: state, err: err}
	}
}

func (m *Model) drainNode(item any) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for draining", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for draining", "item", item)
		return nil
	}
	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	m.logger.Infow("action started", "action", "drainNode", "node", itemKeyString(itemKey))
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := k8s.DrainNode(ctx, m.kubeConfig, m.environment.KubeContext(), node.Name)
		return drainNodeResultMsg{key: itemKey, err: err}
	}
}

// selectedItem returns the currently selected item in the table.
func (m *Model) selectedItem() any {
	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	return findItem(m.dataset, m.category, itemKey)
}
