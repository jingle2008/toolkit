package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
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

// metricsOpenErrMsg reports a failure to launch the metrics dashboard.
type metricsOpenErrMsg struct{ err error }

// metricsWindow is how far back the DAC metrics dashboard looks.
const metricsWindow = 7 * 24 * time.Hour

// pendingMetrics tracks a metrics-open request waiting on the model
// catalogs to load. gen is the load generation it expects, so a later
// unrelated load (e.g. category navigation) cannot resume it.
type pendingMetrics struct {
	ocid      string
	modelName string
	gen       int
}

// openDacMetrics resolves the selected DAC's model capability and opens
// the matching OCI Telemetry MQL dashboard. When the base/imported model
// catalogs aren't loaded yet (the DAC list view loads neither), it kicks
// off the same loads the BaseModel/ImportedModel categories use — so the
// fetched data is cached for later navigation — and resumes the open once
// they arrive. Non-DAC selections are a logged no-op.
func (m *Model) openDacMetrics(item any) tea.Cmd {
	dac, ok := item.(*models.DedicatedAICluster)
	if !ok || dac == nil {
		m.logger.Errorw("no dedicated AI cluster selected for metrics", "category", m.category)
		return nil
	}
	ocid := dac.OCID(m.environment.Realm, m.environment.Region)
	if dac.ModelName == "" {
		return m.launchMetrics(ocid, telemetry.CapabilityChat)
	}
	m.metricsPending = &pendingMetrics{ocid: ocid, modelName: dac.ModelName}
	return m.advanceMetrics()
}

// advanceMetrics either resolves the pending DAC's capability from the
// already-loaded catalogs and launches the dashboard, or dispatches the
// next catalog load — base first, then imported only if the model isn't a
// base model. Returns nil when nothing is pending.
func (m *Model) advanceMetrics() tea.Cmd {
	p := m.metricsPending
	if p == nil {
		return nil
	}
	if m.dataset == nil || m.dataset.BaseModels == nil {
		return m.loadCatalogForMetrics(domain.BaseModel)
	}
	if model := (&models.Dataset{BaseModels: m.dataset.BaseModels}).FindModelByName(p.modelName); model != nil {
		return m.launchPendingMetrics(capabilityForModel(model))
	}
	if m.dataset.ImportedModelMap == nil {
		return m.loadCatalogForMetrics(domain.ImportedModel)
	}
	model := (&models.Dataset{ImportedModelMap: m.dataset.ImportedModelMap}).FindModelByName(p.modelName)
	return m.launchPendingMetrics(capabilityForModel(model))
}

// loadCatalogForMetrics dispatches the shared loader for one model catalog
// (populating m.dataset via the normal *LoadedMsg handlers) and records
// the load generation on the pending request so resumeMetrics only
// continues for this load.
func (m *Model) loadCatalogForMetrics(cat domain.Category) tea.Cmd {
	gen := m.bumpGen()
	m.metricsPending.gen = gen
	var load tea.Cmd
	switch cat { //nolint:exhaustive // only the two model catalogs are loadable here
	case domain.BaseModel:
		load = loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.ImportedModel:
		load = loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return tea.Batch(m.beginTask(), load)
}

// resumeMetrics continues a pending metrics-open after a catalog load, but
// only for the generation the pending request is waiting on — so a load
// triggered by navigation never resumes it.
func (m *Model) resumeMetrics(gen int) tea.Cmd {
	if m.metricsPending == nil || m.metricsPending.gen != gen {
		return nil
	}
	return m.advanceMetrics()
}

// launchPendingMetrics clears the pending request and opens the dashboard.
func (m *Model) launchPendingMetrics(capability telemetry.Capability) tea.Cmd {
	ocid := m.metricsPending.ocid
	m.metricsPending = nil
	return m.launchMetrics(ocid, capability)
}

// launchMetrics opens the dashboard URL in the browser off the UI
// goroutine, reporting a launch failure as an error toast.
func (m *Model) launchMetrics(ocid string, capability telemetry.Capability) tea.Cmd {
	target := metricsURL(m.environment, ocid, capability, time.Now())
	return func() tea.Msg {
		if err := actions.OpenURL(target); err != nil {
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
