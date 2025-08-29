package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateLoadingView handles command routing while in LoadingView mode.
func (m *Model) updateLoadingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
	case DataMsg:
		cmds = append(cmds, m.handleDataMsg(msg))
	case baseModelsLoadedMsg:
		cmds = append(cmds, m.handleBaseModelsLoaded(msg.Items))
	case gpuPoolsLoadedMsg:
		cmds = append(cmds, m.handleGpuPoolsLoaded(msg.Items))
	case gpuNodesLoadedMsg:
		cmds = append(cmds, m.handleGpuNodesLoaded(msg.Items))
	case dedicatedAIClustersLoadedMsg:
		cmds = append(cmds, m.handleDedicatedAIClustersLoaded(msg.Items))
	case tenancyOverridesLoadedMsg:
		cmds = append(cmds, m.handleTenancyOverridesLoaded(msg.Group))
	case limitRegionalOverridesLoadedMsg:
		cmds = append(cmds, m.handleLimitRegionalOverridesLoaded(msg.Items))
	case consolePropertyRegionalOverridesLoadedMsg:
		cmds = append(cmds, m.handleConsolePropertyRegionalOverridesLoaded(msg.Items))
	case propertyRegionalOverridesLoadedMsg:
		cmds = append(cmds, m.handlePropertyRegionalOverridesLoaded(msg.Items))
	case ErrMsg:
		m.handleErrMsg(msg)
	case spinner.TickMsg:
		cmds = append(cmds, m.handleSpinnerTickMsg(msg))
	}
	cmds = append(cmds, m.handleStopwatchMsg(msg))
	return m, tea.Batch(cmds...)
}

func (m *Model) handleErrMsg(msg ErrMsg) {
	m.err = msg
	m.endTask(false)
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
