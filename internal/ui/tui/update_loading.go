package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateLoadingView handles command routing while in LoadingView mode.
func (m *Model) updateLoadingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds, quit := m.routeLoadingMsg(msg)
	if quit {
		return m, tea.Quit
	}
	cmds = append(cmds, m.handleStopwatchMsg(msg))
	return m, tea.Batch(cmds...)
}

func (m *Model) routeLoadingMsg(msg tea.Msg) ([]tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			m.cancelInFlight()
			return nil, true
		}
		return nil, false
	case DataMsg:
		return []tea.Cmd{m.handleDataMsg(msg)}, false
	case datasetLoadedMsg:
		return []tea.Cmd{m.handleDataMsg(DataMsg{Data: msg.Dataset, Gen: msg.Gen})}, false
	case ErrMsg:
		m.handleErrMsg(msg)
		return nil, false
	case spinner.TickMsg:
		return []tea.Cmd{m.handleSpinnerTickMsg(msg)}, false
	default:
		return m.routeLoadingDataMsg(msg), false
	}
}

func (m *Model) routeLoadingDataMsg(msg tea.Msg) []tea.Cmd {
	switch msg := msg.(type) {
	case baseModelsLoadedMsg:
		m.handleBaseModelsLoaded(msg.Items, msg.Gen)
	case gpuPoolsLoadedMsg:
		return []tea.Cmd{m.handleGpuPoolsLoaded(msg.Items, msg.Gen)}
	case gpuNodesLoadedMsg:
		m.handleGpuNodesLoaded(msg.Items, msg.Gen)
	case dedicatedAIClustersLoadedMsg:
		m.handleDedicatedAIClustersLoaded(msg.Items, msg.Gen)
	case tenancyOverridesLoadedMsg:
		m.handleTenancyOverridesLoaded(msg.Group, msg.Gen)
	case limitRegionalOverridesLoadedMsg:
		m.handleLimitRegionalOverridesLoaded(msg.Items, msg.Gen)
	case consolePropertyRegionalOverridesLoadedMsg:
		m.handleConsolePropertyRegionalOverridesLoaded(msg.Items, msg.Gen)
	case propertyRegionalOverridesLoadedMsg:
		m.handlePropertyRegionalOverridesLoaded(msg.Items, msg.Gen)
	default:
		// Future-proof: ignore unknown message types
	}
	return nil
}

func (m *Model) handleSpinnerTickMsg(msg spinner.TickMsg) tea.Cmd {
	loadingSpinner, cmd := m.loadingSpinner.Update(msg)
	m.loadingSpinner = &loadingSpinner
	return cmd
}

func (m *Model) handleStopwatchMsg(msg tea.Msg) tea.Cmd {
	timer, cmd := m.loadingTimer.Update(msg)
	m.loadingTimer = &timer
	return cmd
}
