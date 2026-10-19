package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/internal/mutation"
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
		return m.requestConfirm(m.confirmCordon(itemKey, item))
	case key.Matches(msg, keys.DrainNode):
		return m.requestConfirm(m.confirmDrain(itemKey))
	case key.Matches(msg, keys.Delete):
		return m.requestDelete(itemKey, item)
	case key.Matches(msg, keys.Restart):
		return m.requestConfirm(m.confirmRestart(itemKey, item))
	case key.Matches(msg, keys.ScaleUp):
		return m.requestConfirm(m.confirmScale(itemKey))
	}
	return nil
}

// requestDelete starts ctrl+x. Only an orphaned InferenceService can be
// deleted, and whether that's a delete or a force-delete depends on the
// cluster, so it is checked first and the modal opens with the answer
// (handleISVCDeletePlannedMsg); other categories confirm at once.
func (m *Model) requestDelete(itemKey models.ItemKey, item any) tea.Cmd {
	if m.category == domain.InferenceService {
		return m.planDeleteInferenceService(itemKey, item)
	}
	return m.requestConfirm(m.confirmDelete(itemKey, ""))
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
	env := m.environment
	return func() tea.Msg {
		actions.CopyTenantID(item, env, m.logger)
		return nil
	}
}

func (m *Model) scaleUpGPUPool(item any, itemKey models.ItemKey) tea.Cmd {
	pool, ok := item.(*models.GPUPool)
	if !ok || pool == nil {
		m.logger.Errorw("no GPU pool selected for scale up")
		return nil
	}
	env := m.environment
	return m.trackAction("scale up "+pool.Name, "Scaling", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, mutation.ScaleGPUPool, pool.Name, func() error {
			return oci.IncreasePoolSize(ctx, pool, env, m.logger)
		})
		return gpuPoolScaleResultMsg{key: itemKey, err: err, env: env}
	})
}

func (m *Model) toggleFaultyList() tea.Cmd {
	m.showFaulty = !m.showFaulty
	return m.updateRowsAsync()
}

// cordonNode sets the node's cordon as a (CordonNode or UncordonNode)
// asks, as the CLI and MCP do; a node already there is left as is.
func (m *Model) cordonNode(item any, itemKey models.ItemKey, a mutation.Action) tea.Cmd {
	if item == nil {
		m.logger.Errorw("no item selected for cordon operation", "category", m.category)
		return nil
	}
	node, ok := item.(*models.GPUNode)
	if !ok {
		m.logger.Errorw("unsupported item type for cordon operation", "item", item)
		return nil
	}
	env, kubeConfig, name := m.environment, m.kubeConfig, node.Name
	unschedulable := a == mutation.CordonNode
	return m.trackAction(strings.ToLower(confirmLabels[a].verb)+" "+name, "", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		err := audited(m.logger, a, name, func() error {
			_, err := setCordonFn(ctx, kubeConfig, env.KubeContext(), name, unschedulable)
			return err
		})
		return cordonNodeResultMsg{key: itemKey, state: unschedulable, err: err, env: env}
	})
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
	env, kubeConfig, name := m.environment, m.kubeConfig, node.Name
	return m.trackAction("drain "+name, "Draining", itemKey, func(progress func(string)) tea.Msg {
		// Draining restarts single-replica services and waits for them to
		// reschedule, which can take tens of minutes under its own internal
		// timeouts; the 30s one-shot cap would abort it midway.
		ctx := k8s.WithDrainProgress(m.longOpCtx(), func(line string) {
			progress(strings.TrimPrefix(line, "drain: "))
		})
		err := audited(m.logger, mutation.DrainNode, name, func() error {
			return k8s.DrainNode(ctx, kubeConfig, env.KubeContext(), name)
		})
		return drainNodeResultMsg{key: itemKey, err: err, env: env}
	})
}

// selectedItem returns the currently selected item in the table.
func (m *Model) selectedItem() any {
	itemKey := itemKeyFrom(m.category, m.selectedRawRow())
	return findItem(m.dataset, m.category, itemKey)
}

// metricsOpenErrMsg reports a failure to launch the metrics dashboard.
type metricsOpenErrMsg struct{ err error }

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
// (DAC, ImportedModel, GPUWorkload, or InferenceService). If the plan needs a model catalog
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

// pickMetricsDAC opens the picker over the DACs serving an imported model
// (one per copy in a DAC namespace). Choosing one opens metrics for the
// model narrowed to that copy, so the single-DAC plan applies.
func (m *Model) pickMetricsDAC(im *models.ImportedModel, dacs []string) tea.Cmd {
	targets := make([]domain.Scope, len(dacs))
	for i, ns := range dacs {
		targets[i] = domain.Scope{Category: domain.DedicatedAICluster, Name: ns}
	}
	return m.openRowPicker(fmt.Sprintf("Open metrics for %s on", im.Name), "open metrics", targets, func(t domain.Scope) tea.Cmd {
		one, err := telemetry.OnDAC(*im, t.Name)
		if err != nil {
			return m.showToast(err.Error(), toastError)
		}
		return m.openMetrics(&one)
	})
}

// metricsCatalog reports which model catalog must be loaded before the
// item's plan can be resolved (telemetry.CatalogFor).
func (m *Model) metricsCatalog(item any) (domain.Category, bool) {
	return telemetry.CatalogFor(item)
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
	if msg.gen != m.gens.msg || !m.catalogLoaded(msg.cat) {
		return nil
	}
	return m.finishMetrics(msg.item)
}

// finishMetrics resolves the item's plan and either launches the dashboard
// or shows an error toast. A no-op item (empty reason) yields nil. An
// imported model served by several DACs first goes through the DAC
// picker, so resolveMetricsPlan — called only from here — sees one.
func (m *Model) finishMetrics(item any) tea.Cmd {
	if im, ok := item.(*models.ImportedModel); ok && im != nil {
		if dacs := im.DACNamespaces(); len(dacs) > 1 {
			return m.pickMetricsDAC(im, dacs)
		}
	}
	filter, capability, ok, reason := m.resolveMetricsPlan(item)
	if !ok {
		if reason == "" {
			return nil
		}
		return m.showToast(reason, toastWarn) // the row has no dashboard: not a failure
	}
	return m.launchMetrics(filter, capability)
}

// resolveMetricsPlan maps a selected item to its metrics plan
// (telemetry.PlanFor, shared with `toolkit metrics` and MCP's
// metrics_url): the MQL filter and capability, plus ok/reason. ok=false
// with a non-empty reason is a user-facing error toast; ok=false with
// empty reason is a silent no-op (unknown/nil item). Reads m.dataset,
// which is non-nil whenever a catalog was required (guaranteed loaded
// before this runs).
func (m *Model) resolveMetricsPlan(item any) (telemetry.Filter, telemetry.Capability, bool, string) {
	var catalog telemetry.Catalog
	if m.dataset != nil {
		catalog = m.dataset
	}
	plan, err := telemetry.PlanFor(item, m.environment, catalog)
	switch {
	case errors.Is(err, telemetry.ErrNoMetrics):
		return telemetry.Filter{}, telemetry.CapabilityChat, false, ""
	case err != nil:
		return telemetry.Filter{}, telemetry.CapabilityChat, false, err.Error()
	default:
		return plan.Filter, plan.Capability, true, ""
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
	return telemetry.DashboardURL(env, telemetry.Plan{Filter: filter, Capability: capability}, now)
}
