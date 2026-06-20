package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
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
	case key.Matches(msg, keys.EditTenant):
		return m.enterEditTenantView()
	case key.Matches(msg, keys.OpenMetrics):
		return m.openDacMetrics(item)
	case key.Matches(msg, keys.Refresh):
		return tea.Sequence(m.updateCategoryNoHist(m.category)...)
	case key.Matches(msg, keys.ToggleCordon):
		return m.cordonNode(item, itemKey)
	case key.Matches(msg, keys.DrainNode):
		return m.drainNode(item, itemKey)
	case key.Matches(msg, keys.Delete):
		return m.deleteItem(itemKey)
	case key.Matches(msg, keys.RebootNode):
		return m.rebootNode(item, itemKey)
	case key.Matches(msg, keys.ScaleUp):
		return m.scaleUpGPUPool(item, itemKey)
	}
	return nil
}

func (m *Model) copyTenantID(item any) tea.Cmd {
	return func() tea.Msg {
		actions.CopyTenantID(item, m.environment, m.logger)
		return nil
	}
}

func (m *Model) scaleUpGPUPool(item any, itemKey models.ItemKey) tea.Cmd {
	pool, ok := item.(*models.GPUPool)
	if !ok || pool == nil {
		m.logger.Errorw("no GPU pool selected for scale up")
		return nil
	}

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

func (m *Model) cordonNode(item any, itemKey models.ItemKey) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for cordon operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for cordon operation", "item", item)
		return nil
	}
	m.logger.Infow("action started", "action", "toggleCordon", "node", itemKeyString(itemKey))
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		state, err := k8s.ToggleCordon(ctx, m.kubeConfig, m.environment.KubeContext(), node.Name)
		return cordonNodeResultMsg{key: itemKey, state: state, err: err}
	}
}

func (m *Model) drainNode(item any, itemKey models.ItemKey) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for draining", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for draining", "item", item)
		return nil
	}
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

// metricsOpenErrMsg reports a failure to resolve the model capability or
// launch the metrics dashboard.
type metricsOpenErrMsg struct{ err error }

// metricsWindow is how far back the DAC metrics dashboard looks.
const metricsWindow = 7 * 24 * time.Hour

// metricsResolveTimeout bounds the on-demand model-catalog loads done
// while resolving a DAC's capability.
const metricsResolveTimeout = 30 * time.Second

// openDacMetrics resolves the selected DAC's model capability and opens
// the matching OCI Telemetry MQL dashboard in the browser, off the UI
// goroutine. Resolving the capability may lazily load the base/imported
// model catalogs, because the DAC list view does not load them; a load
// failure surfaces as an error toast and the browser is NOT opened, since
// the dashboard could otherwise show the wrong metrics. Non-DAC
// selections are a logged no-op.
func (m *Model) openDacMetrics(item any) tea.Cmd {
	dac, ok := item.(*models.DedicatedAICluster)
	if !ok || dac == nil {
		m.logger.Errorw("no dedicated AI cluster selected for metrics", "category", m.category)
		return nil
	}
	// Snapshot inputs on the UI goroutine. Reusing already-loaded model
	// catalogs avoids a fetch; we only hit the loader when they are nil.
	var (
		env       = m.environment
		ocid      = dac.OCID(env.Realm, env.Region)
		modelName = dac.ModelName
		ld        = m.loader
		kubeCfg   = m.kubeConfig
		parent    = m.parentCtx
		now       = time.Now()
		base      []models.BaseModel
		imported  map[string][]models.ImportedModel
	)
	if m.dataset != nil {
		base = m.dataset.BaseModels
		imported = m.dataset.ImportedModelMap
	}
	return func() tea.Msg {
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, metricsResolveTimeout)
		defer cancel()
		capability, err := resolveCapability(ctx, ld, kubeCfg, env, modelName, base, imported)
		if err != nil {
			return metricsOpenErrMsg{err: err}
		}
		if err := actions.OpenURL(metricsURL(env, ocid, capability, now)); err != nil {
			return metricsOpenErrMsg{err: err}
		}
		return nil
	}
}

// metricsURL builds the OCI Telemetry MQL dashboard URL for a resolved
// capability, from the environment (realm/region/type) and a window
// ending at now. Pure; unit-testable without launching a browser.
func metricsURL(env models.Environment, ocid string, capability telemetry.Capability, now time.Time) string {
	fleet := "generative-ai-service-api-" + env.Type
	return telemetry.MetricsURL(ocid, capability, env.Region, telemetry.Project, fleet,
		now.Add(-metricsWindow), now)
}

// Model capability strings as they appear in BaseModel.Capabilities
// (sourced from the K8s CR's spec.modelCapabilities).
const (
	capabilityChat       = "CHAT"
	capabilityTextRerank = "TEXT_RERANK"
	capabilityTextEmbed  = "TEXT_EMBEDDINGS"
)

// resolveCapability resolves modelName to its metric capability, lazily
// loading the base catalog and then the imported catalog only when the
// caller did not already supply them (the DAC list view loads neither).
// It returns an error only when a needed load fails — the caller then
// declines to open a possibly-wrong dashboard. A model that loads
// cleanly but isn't found, is a finetune, or has no recognized
// capability resolves to CapabilityChat.
func resolveCapability(
	ctx context.Context,
	ld loader.Composite,
	kubeCfg string,
	env models.Environment,
	modelName string,
	base []models.BaseModel,
	imported map[string][]models.ImportedModel,
) (telemetry.Capability, error) {
	if modelName == "" {
		return telemetry.CapabilityChat, nil
	}
	if base == nil {
		loaded, err := ld.LoadBaseModels(ctx, kubeCfg, env)
		if err != nil {
			return telemetry.CapabilityChat, fmt.Errorf("loading base models: %w", err)
		}
		base = loaded
	}
	model := (&models.Dataset{BaseModels: base}).FindModelByName(modelName)
	if model == nil {
		if imported == nil {
			loaded, err := ld.LoadImportedModels(ctx, kubeCfg, env)
			if err != nil {
				return telemetry.CapabilityChat, fmt.Errorf("loading imported models: %w", err)
			}
			imported = loaded
		}
		model = (&models.Dataset{ImportedModelMap: imported}).FindModelByName(modelName)
	}
	return capabilityForModel(model), nil
}

// capabilityForModel maps a resolved model to its metric capability,
// defaulting to chat for nil/finetune/unrecognized. Precedence when
// several capabilities are present: CHAT > TEXT_RERANK > TEXT_EMBEDDINGS.
func capabilityForModel(model *models.BaseModel) telemetry.Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return telemetry.CapabilityChat
	}
	switch {
	case model.HasCapability(capabilityChat):
		return telemetry.CapabilityChat
	case model.HasCapability(capabilityTextRerank):
		return telemetry.CapabilityTextRerank
	case model.HasCapability(capabilityTextEmbed):
		return telemetry.CapabilityTextEmbeddings
	default:
		return telemetry.CapabilityChat
	}
}
