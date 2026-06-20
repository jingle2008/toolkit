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

// importedModelNamePrefix is the OCID resource-id prefix carried by
// tenant-owned imported and finetune model names. A DAC ModelName with
// this prefix lives in the imported-model catalog; anything else is a
// public base model — so we load only the one catalog we need.
const importedModelNamePrefix = "amaaaaaa"

// openMetricsTriggerMsg is the second step of openDacMetrics's sequence:
// it fires after the model catalog has been loaded and applied, so its
// handler reads the now-populated dataset on the Update loop. gen pins it
// to the load it followed.
type openMetricsTriggerMsg struct {
	ocid      string
	modelName string
	cat       domain.Category
	gen       int
}

// openDacMetrics resolves the selected DAC's model capability and opens
// the matching OCI Telemetry MQL dashboard. The ModelName picks the single
// catalog to consult — imported/finetune names carry importedModelNamePrefix,
// everything else is a base model. If that catalog is already loaded the
// link opens immediately; otherwise the catalog is fetched (and cached on
// the dataset for later navigation) before the link opens. Non-DAC
// selections are a logged no-op.
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
	cat := domain.BaseModel
	if strings.HasPrefix(dac.ModelName, importedModelNamePrefix) {
		cat = domain.ImportedModel
	}
	if m.catalogLoaded(cat) {
		return m.launchMetrics(ocid, m.modelCapability(dac.ModelName))
	}
	// Load the catalog through the shared category loader (so it is cached
	// for later navigation, exactly as a category load would), then fire
	// the open trigger. tea.Sequence guarantees the catalog's *LoadedMsg is
	// applied on the Update loop before the trigger message is handled —
	// the same ordering Init relies on for tenant resolution — so the
	// trigger's handler sees the populated dataset.
	gen := m.bumpGen()
	return tea.Sequence(
		tea.Batch(m.beginTask(), m.catalogLoadCmd(cat, gen)),
		func() tea.Msg {
			return openMetricsTriggerMsg{ocid: ocid, modelName: dac.ModelName, cat: cat, gen: gen}
		},
	)
}

// catalogLoaded reports whether the given model catalog is already present
// on the dataset (nil means "not loaded yet").
func (m *Model) catalogLoaded(cat domain.Category) bool {
	if m.dataset == nil {
		return false
	}
	switch cat { //nolint:exhaustive // only the two model catalogs are relevant here
	case domain.ImportedModel:
		return m.dataset.ImportedModelMap != nil
	default:
		return m.dataset.BaseModels != nil
	}
}

// modelCapability resolves a model name against the loaded dataset to its
// metric capability (chat for unresolved / finetune / unrecognized).
func (m *Model) modelCapability(modelName string) telemetry.Capability {
	if m.dataset == nil {
		return telemetry.CapabilityChat
	}
	return capabilityForModel(m.dataset.FindModelByName(modelName))
}

// catalogLoadCmd returns the shared loader command for one model catalog;
// its *LoadedMsg is applied by the normal handler, caching the catalog on
// the dataset for later navigation.
func (m *Model) catalogLoadCmd(cat domain.Category, gen int) tea.Cmd {
	switch cat { //nolint:exhaustive // only the two model catalogs are loadable here
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	default:
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
}

// handleOpenMetricsTrigger opens the dashboard once its catalog load has
// been applied. It declines when the generation is stale (the user
// navigated) or the catalog still isn't loaded (the load failed or was
// dropped — its own error toast already fired), so a possibly-wrong
// dashboard never opens.
func (m *Model) handleOpenMetricsTrigger(msg openMetricsTriggerMsg) tea.Cmd {
	if msg.gen != m.gen || !m.catalogLoaded(msg.cat) {
		return nil
	}
	return m.launchMetrics(msg.ocid, m.modelCapability(msg.modelName))
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
