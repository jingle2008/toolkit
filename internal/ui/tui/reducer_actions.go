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
		return m.openMetrics(item)
	case key.Matches(msg, keys.Refresh):
		return m.handleRefresh()
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

// handleRefresh reloads the current category (re-establishing its k8s watch)
// and, since the repo watch has no auto-reconnect, recovers it too when it has
// dropped.
func (m *Model) handleRefresh() tea.Cmd {
	cmds := m.updateCategoryNoHist(m.category)
	if c := m.maybeStartRepoWatchCmd(); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Sequence(cmds...)
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

// metricsWindow is how far back the metrics dashboard looks.
const metricsWindow = 7 * 24 * time.Hour

// importedModelNamePrefix is the OCID resource-id prefix carried by
// tenant-owned imported/finetune model names and by DAC names. A model name
// with this prefix resolves against the imported catalog, anything else
// against the base catalog. A workload/imported-model namespace with this
// prefix is a DAC name.
const importedModelNamePrefix = "amaaaaaa"

// openMetricsTriggerMsg is the second step of openMetrics's sequence: it
// fires after the model catalog has been loaded and applied, so its handler
// resolves the plan against the now-populated dataset on the Update loop. gen
// pins it to the load it followed; item is the selection to resolve.
type openMetricsTriggerMsg struct {
	item any
	cat  domain.Category
	gen  int
}

// openMetrics opens the OCI Telemetry MQL dashboard for the selected item
// (DAC, ImportedModel, or GPUWorkload). If the plan needs a model catalog
// that isn't loaded yet, the catalog is fetched (and cached for later
// navigation) before the dashboard opens. Items that can't produce metrics
// are a no-op or an error toast (resolveMetricsPlan).
func (m *Model) openMetrics(item any) tea.Cmd {
	cat, need := m.metricsCatalog(item)
	if !need || m.catalogLoaded(cat) {
		return m.finishMetrics(item)
	}
	gen := m.bumpGen()
	return tea.Sequence(
		tea.Batch(m.beginTask(), m.catalogLoadCmd(cat, gen)),
		func() tea.Msg { return openMetricsTriggerMsg{item: item, cat: cat, gen: gen} },
	)
}

// metricsCatalog reports which model catalog must be loaded before the
// item's plan can be resolved. need is false when no catalog is required
// (DAC without a model) or the item can't produce metrics here (default).
// Task 4 adds the ImportedModel and GPUWorkload cases.
func (m *Model) metricsCatalog(item any) (domain.Category, bool) {
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil || it.ModelName == "" {
			return domain.BaseModel, false
		}
		return modelCatalog(it.ModelName), true
	case *models.GPUWorkload:
		if it == nil || it.Model == "" {
			return domain.BaseModel, false
		}
		if strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			return modelCatalog(it.Model), true // dedicated
		}
		return domain.BaseModel, true // on-demand matches the base catalog
	default: // *models.ImportedModel (capabilities inline) or unknown
		return domain.BaseModel, false
	}
}

// modelCatalog routes a model NAME to the catalog that holds it: imported/
// finetune names carry importedModelNamePrefix, everything else is base.
func modelCatalog(modelName string) domain.Category {
	if strings.HasPrefix(modelName, importedModelNamePrefix) {
		return domain.ImportedModel
	}
	return domain.BaseModel
}

// catalogLoaded reports whether the given model catalog is present on the
// dataset (nil means not loaded yet).
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

// catalogLoadCmd returns the shared loader command for one model catalog; its
// *LoadedMsg is applied by the normal handler, caching the catalog on the
// dataset for later navigation.
func (m *Model) catalogLoadCmd(cat domain.Category, gen int) tea.Cmd {
	switch cat { //nolint:exhaustive // only the two model catalogs are loadable here
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	default:
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
}

// handleOpenMetricsTrigger resolves and opens the dashboard once the catalog
// load has been applied. It declines on a stale generation (a later load
// superseded this one) or when the catalog still isn't loaded (load failed —
// its errMsg toast already fired — or was stale-dropped).
func (m *Model) handleOpenMetricsTrigger(msg openMetricsTriggerMsg) tea.Cmd {
	if msg.gen != m.gen || !m.catalogLoaded(msg.cat) {
		return nil
	}
	return m.finishMetrics(msg.item)
}

// finishMetrics resolves the item's plan and either launches the dashboard
// or shows an error toast. A no-op item (empty reason) yields nil.
func (m *Model) finishMetrics(item any) tea.Cmd {
	filter, capability, ok, reason := m.resolveMetricsPlan(item)
	if !ok {
		if reason == "" {
			return nil
		}
		return m.showToast(reason, toastError)
	}
	return m.launchMetrics(filter, capability)
}

// resolveMetricsPlan maps a selected item to its metrics plan: the MQL filter
// and capability, plus ok/reason. ok=false with a non-empty reason is a
// user-facing error toast; ok=false with empty reason is a silent no-op
// (unknown/nil item). Reads m.dataset, which is non-nil whenever a catalog
// was required (guaranteed loaded before this runs). Task 4 adds the
// ImportedModel and GPUWorkload cases.
//
//nolint:cyclop // per-item-type metrics resolution; the switch is the routing surface
func (m *Model) resolveMetricsPlan(item any) (telemetry.Filter, telemetry.Capability, bool, string) {
	realm, region := m.environment.Realm, m.environment.Region
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, ""
		}
		filter := telemetry.Filter{Key: telemetry.FilterDacID, Value: it.OCID(realm, region)}
		if it.ModelName == "" {
			return filter, telemetry.CapabilityChat, true, ""
		}
		return m.dedicatedPlan(filter, m.dataset.FindModelByName(it.ModelName))
	case *models.ImportedModel:
		if it == nil {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, ""
		}
		if !strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "imported model is not tied to a dedicated AI cluster"
		}
		ocid := models.DedicatedAICluster{Name: it.Namespace}.OCID(realm, region)
		filter := telemetry.Filter{Key: telemetry.FilterDacID, Value: ocid}
		return m.dedicatedPlan(filter, &it.BaseModel)
	case *models.GPUWorkload:
		if it == nil {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, ""
		}
		if it.Model == "" {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "workload has no model"
		}
		if strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			ocid := models.DedicatedAICluster{Name: it.Namespace}.OCID(realm, region)
			filter := telemetry.Filter{Key: telemetry.FilterDacID, Value: ocid}
			return m.dedicatedPlan(filter, m.dataset.FindModelByName(it.Model))
		}
		bm := m.dataset.FindBaseModelByName(it.Model)
		if bm == nil {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "model not found in base catalog"
		}
		capability := capabilityForModel(bm)
		if !capability.Supported() {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not supported for this model"
		}
		return telemetry.Filter{Key: telemetry.FilterResourceID, Value: bm.DisplayName}, capability, true, ""
	default:
		return telemetry.Filter{}, telemetry.CapabilityChat, false, ""
	}
}

// dedicatedPlan finalizes a DacId-filtered (dedicated-mode) plan. A capability
// with no dashboard at all is unsupported; an unfilterable capability (the
// fixed, unfiltered moderation sets) cannot be DacId-scoped, so it has no
// dedicated-mode dashboard. Either yields a no-op error toast.
func (m *Model) dedicatedPlan(filter telemetry.Filter, model *models.BaseModel) (telemetry.Filter, telemetry.Capability, bool, string) {
	capability := capabilityForModel(model)
	switch {
	case !capability.Supported():
		return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not supported for this model"
	case !capability.Filterable():
		return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not available for this model in dedicated mode"
	default:
		return filter, capability, true, ""
	}
}

// launchMetrics opens the dashboard URL in the browser off the UI goroutine,
// reporting a launch failure as an error toast.
func (m *Model) launchMetrics(filter telemetry.Filter, capability telemetry.Capability) tea.Cmd {
	target := metricsURL(m.environment, filter, capability, time.Now())
	return func() tea.Msg {
		if err := actions.OpenURL(target); err != nil {
			return metricsOpenErrMsg{err: err}
		}
		return nil
	}
}

// metricsURL builds the OCI Telemetry MQL dashboard URL from the environment
// (region/type), an MQL filter, a capability, and a window ending at now.
// Pure; unit-testable without launching a browser.
func metricsURL(env models.Environment, filter telemetry.Filter, capability telemetry.Capability, now time.Time) string {
	fleet := "generative-ai-service-api-" + env.Type
	return telemetry.MetricsURL(filter, capability, env.Region, telemetry.Project, fleet,
		now.Add(-metricsWindow), now)
}

// capabilityPrecedence maps a model-capability string to its telemetry
// capability, in priority order: the first capability the model declares
// wins. CHAT is highest; the four unsupported capabilities are last, so any
// supported capability outranks them (a purely-unsupported model resolves to
// CapabilityUnsupported). Synonyms (EMBEDDING, CONTENT_MODERATION,
// IMAGE_TEXT_TO_TEXT) map to the same telemetry capability as their primary.
var capabilityPrecedence = []struct {
	flag string
	cap  telemetry.Capability
}{
	{models.CapabilityChat, telemetry.CapabilityChat},
	{models.CapabilityTextToText, telemetry.CapabilityChat},
	{models.CapabilityImageTextToText, telemetry.CapabilityChat},
	{models.CapabilityTextRerank, telemetry.CapabilityTextRerank},
	{models.CapabilityTextEmbeddings, telemetry.CapabilityTextEmbeddings},
	{models.CapabilityEmbedding, telemetry.CapabilityTextEmbeddings},
	{models.CapabilityTextToImage, telemetry.CapabilityTextToImage},
	{models.CapabilityImageTextToImage, telemetry.CapabilityImageTextToImage},
	{models.CapabilityTextToAudio, telemetry.CapabilityTextToAudio},
	{models.CapabilityAudioToText, telemetry.CapabilityAudioToText},
	{models.CapabilityTextClassification, telemetry.CapabilityTextClassification},
	{models.CapabilityContentModeration, telemetry.CapabilityTextClassification},
	{models.CapabilityImageContentModeration, telemetry.CapabilityImageContentModeration},
	{models.CapabilityTextGeneration, telemetry.CapabilityUnsupported},
	{models.CapabilityAudioToAudio, telemetry.CapabilityUnsupported},
	{models.CapabilityRealtime, telemetry.CapabilityUnsupported},
	{models.CapabilityPromptInjectionProtection, telemetry.CapabilityUnsupported},
}

// capabilityForModel maps a resolved model to its metric capability via
// capabilityPrecedence (first declared match wins). nil / finetune / a model
// declaring no recognized capability fall back to CapabilityChat.
func capabilityForModel(model *models.BaseModel) telemetry.Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return telemetry.CapabilityChat
	}
	for _, p := range capabilityPrecedence {
		if model.HasCapability(p.flag) {
			return p.cap
		}
	}
	return telemetry.CapabilityChat
}
