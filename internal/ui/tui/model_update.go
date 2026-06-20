/*
Package tui implements the update/reduce logic for the Model.
*/
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

/*
Update implements the tea.Model interface and updates the Model state in response to a message.
*/
//
//nolint:cyclop // top-level message router; the per-message-type switch is intentionally exhaustive — splitting it into helpers would obscure the routing surface without reducing real complexity.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)
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
	case stopwatch.TickMsg, stopwatch.StartStopMsg, stopwatch.ResetMsg:
		return m, m.handleStopwatchMsg(msg)
	// Data / loaded messages: routed at the top so a load completing
	// while the user has navigated into DetailsView/HelpView/ExportView
	// still updates the dataset and drains endTask — without this,
	// pendingTasks would stay elevated and the inline spinner would
	// tick forever.
	case dataMsg:
		return m, m.handleDataMsg(msg)
	case datasetLoadedMsg:
		return m, m.handleDataMsg(dataMsg{Data: msg.Dataset, Gen: msg.Gen})
	case baseModelsLoadedMsg, importedModelsLoadedMsg, gpuPoolsLoadedMsg,
		gpuNodesLoadedMsg, gpuWorkloadsLoadedMsg, dedicatedAIClustersLoadedMsg, tenancyOverridesLoadedMsg,
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
	case tableRowsComputedMsg:
		m.handleTableRowsComputedMsg(msg)
		return m, nil
	case detailContentRenderedMsg:
		return m, m.handleDetailContentRenderedMsg(msg)
	default:
		return m.delegateToActiveView(msg)
	}
}

func (m *Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.updateLayout(msg.Width, msg.Height)
	if m.viewMode == common.DetailsView {
		return m, m.updateContentAsync()
	}
	return m, nil
}

func (m *Model) delegateToActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	}
	return m, nil
}
