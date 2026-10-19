/*
Package tui implements the update/reduce logic for the Model.
*/
package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

/*
Update implements the tea.Model interface and updates the Model state in
response to a message. While the keys-changed hint is up, the first key
press dismisses it and is then handled as usual. While an error toast is
up, esc in the list or details view dismisses it instead of backing out.
An open details view re-renders after any message that rebuilt the rows.
*/
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.interceptToastDismiss(msg) {
		return m, nil
	}
	var hint tea.Cmd
	if _, ok := msg.(tea.KeyPressMsg); ok && m.keysHint {
		hint = m.dismissKeysHint()
	}
	model, cmd := m.route(msg)
	return model, tea.Batch(hint, cmd, m.refreshStaleDetail())
}

// route dispatches msg to its handler.
//
//nolint:cyclop // top-level message router; the per-message-type switch is intentionally exhaustive — splitting it into helpers would obscure the routing surface without reducing real complexity.
func (m *Model) route(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)
	case tea.BackgroundColorMsg:
		m.lightBg = !msg.IsDark()
		setStyles(m, DefaultStyles(msg.IsDark()))
		if r, ok := m.renderer.(view.ProductionRenderer); ok {
			r.Light = m.lightBg // the JSON view follows the background
			m.renderer = r
		}
		return m, m.updateContentAsync()
	case tea.ColorProfileMsg:
		m.colorless = msg.Profile == colorprofile.NoTTY || msg.Profile == colorprofile.ASCII
		setStyles(m, DefaultStyles(!m.lightBg))
		return m, nil
	case errMsg:
		return m, m.handleErrMsg(msg)
	case toastExpireMsg:
		m.handleToastExpireMsg(msg)
		return m, nil
	case spinner.TickMsg:
		// Spinner ticks must fire regardless of view mode so the
		// status-bar loading nugget keeps animating while the user
		// stays in ListView/DetailsView during a load.
		return m, m.handleSpinnerTickMsg(msg)
	case logTickMsg:
		if m.viewMode == common.LogView {
			return m, logTickCmd()
		}
		return m, nil
	case stopwatch.TickMsg, stopwatch.StartStopMsg, stopwatch.ResetMsg:
		return m, m.handleStopwatchMsg(msg)
	// Data / loaded messages: routed at the top so a load completing
	// while the user has navigated into DetailsView/HelpView/ExportView
	// still updates the dataset and drains endTask — without this,
	// pendingTasks would stay elevated and the inline spinner would
	// tick forever.
	case dataMsg:
		m.handleDataMsg(msg)
		return m, nil
	case datasetLoadedMsg:
		m.handleDataMsg(dataMsg{Data: msg.Dataset, Gen: msg.Gen})
		return m, m.pruneRecentEnvs()
	case baseModelsLoadedMsg, servingRuntimesLoadedMsg, importedModelsLoadedMsg, gpuPoolsLoadedMsg,
		gpuNodesLoadedMsg, gpuWorkloadsLoadedMsg, dedicatedAIClustersLoadedMsg, inferenceServicesLoadedMsg, tenancyOverridesLoadedMsg,
		limitRegionalOverridesLoadedMsg, consolePropertyRegionalOverridesLoadedMsg,
		propertyRegionalOverridesLoadedMsg:
		return m, tea.Batch(m.routeListLoadedMsg(msg)...)
	// Tenant-save results are intercepted here so they fire from any
	// view: the user may dismiss the form (esc) before the async write
	// lands, otherwise the result would route to the list view and be
	// silently dropped (no toast, no reload).
	case tenantSavedMsg:
		return m, m.handleTenantSavedMsg(msg)
	case tenantSaveErrMsg:
		return m, m.handleTenantSaveErrMsg(msg)
	case portalOpenErrMsg:
		// Intercepted here (not in the form handler) so the toast still
		// fires if the user dismissed the form before the launch failed.
		return m, m.showToast(fmt.Sprintf("failed to open portal: %v", msg.err), toastError)
	case metricsOpenErrMsg:
		return m, m.showToast(fmt.Sprintf("failed to open metrics: %v", msg.err), toastError)
	case openMetricsTriggerMsg:
		return m, m.handleOpenMetricsTrigger(msg)
	// Actions' results are handled from any view: the user may be in
	// details, help or the log when one finishes.
	case actionDoneMsg:
		return m.route(m.handleActionDone(msg))
	case actionProgressMsg:
		return m, m.handleActionProgress(msg)
	case deleteErrMsg:
		return m, m.handleDeleteErrMsg(msg)
	case deleteDoneMsg:
		return m, m.handleDeleteDoneMsg(msg)
	case gpuPoolScaleResultMsg:
		return m, m.handleGPUPoolScaleResultMsg(msg)
	case cordonNodeResultMsg:
		return m, m.handleCordonNodeResultMsg(msg)
	case drainNodeResultMsg:
		return m, m.handleDrainNodeResultMsg(msg)
	case restartNodeResultMsg:
		return m, m.handleRestartNodeResultMsg(msg)
	case restartServiceResultMsg:
		return m, m.handleRestartServiceResultMsg(msg)
	case isvcDeletePlannedMsg:
		return m, m.handleISVCDeletePlannedMsg(msg)
	case isvcDeleteResultMsg:
		return m, m.handleISVCDeleteResultMsg(msg)
	case tableRowsComputedMsg:
		m.handleTableRowsComputedMsg(msg)
		return m, nil
	case detailContentRenderedMsg:
		return m, m.handleDetailContentRenderedMsg(msg)
	case k8sWatchStartedMsg:
		return m, m.handleK8sWatchStarted(msg)
	case k8sWatchTriggeredMsg:
		return m, m.handleK8sWatchTriggered(msg)
	case k8sWatchClosedMsg:
		return m, m.handleK8sWatchClosed(msg)
	case k8sWatchUnavailableMsg:
		m.handleK8sWatchUnavailable(msg)
		return m, nil
	case repoWatchStartedMsg:
		return m, m.handleRepoWatchStarted(msg)
	case repoWatchTriggeredMsg:
		return m, m.handleRepoWatchTriggered()
	case repoWatchClosedMsg:
		m.handleRepoWatchClosed()
		return m, nil
	case datasetReloadedMsg:
		m.handleDatasetReloaded(msg)
		return m, nil
	case gpuPoolsReloadedMsg:
		return m, m.handleGPUPoolsReloaded(msg)
	case kubeVersionMsg:
		m.handleKubeVersionMsg(msg)
		return m, nil
	default:
		return m.delegateToActiveView(msg)
	}
}

// handleKubeVersionMsg records the server version and re-runs layout, since
// the info column's width (and thus the help bubble's budget) may change.
func (m *Model) handleKubeVersionMsg(msg kubeVersionMsg) {
	if msg.Context != m.environment.KubeContext() {
		return
	}
	m.k8sVersion = msg.Version
	m.updateLayout(m.viewWidth, m.viewHeight)
}

func (m *Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.updateLayout(msg.Width, msg.Height)
	if m.viewMode == common.DetailsView {
		return m, m.updateContentAsync()
	}
	return m, nil
}

// interceptToggleLog checks whether msg is the ToggleLog key pressed from a
// view that supports the overlay. If so it transitions to LogView, records the
// return view, and arms the live-refresh tick. Returns (model, cmd, true) on
// intercept, (m, nil, false) otherwise.
func (m *Model) interceptToggleLog(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok || !key.Matches(km, keys.ToggleLog) {
		return m, nil, false
	}
	if m.viewMode != common.ListView && m.viewMode != common.DetailsView {
		return m, nil, false
	}
	// A backtick typed into the filter or alias input is text.
	if m.viewMode == common.ListView && m.inputMode != common.NormalInput {
		return m, nil, false
	}
	m.log.returnView = m.viewMode
	m.viewMode = common.LogView
	return m, logTickCmd(), true
}

func (m *Model) delegateToActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if model, cmd, intercepted := m.interceptToggleLog(msg); intercepted {
		return model, cmd
	}
	switch m.viewMode {
	case common.HelpView:
		return m.updateHelpView(msg)
	case common.ListView:
		return m.updateListView(msg)
	case common.DetailsView:
		return m.updateDetailView(msg)
	case common.LoadingView:
		return m.updateLoadingView(msg)
	case common.ExportView:
		return m.updateExportView(msg)
	case common.EditTenantView:
		return m.updateEditTenantView(msg)
	case common.LogView:
		return m.updateLogView(msg)
	case common.ConfirmView:
		return m.updateConfirmView(msg)
	case common.RowPickerView:
		return m.updateRowPickerView(msg)
	case common.PaletteView:
		return m.updatePaletteView(msg)
	}
	return m, nil
}
