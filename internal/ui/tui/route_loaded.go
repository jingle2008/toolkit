package tui

import tea "github.com/charmbracelet/bubbletea"

// routeLoadedDataMsg dispatches a typed *LoadedMsg to its matching
// handler. The only branch that returns a follow-up cmd is
// gpuPoolsLoadedMsg (state enrichment); all others return nil.
// Shared by updateLoadingView and updateListView so the per-type
// switch lives in one place.
//
//nolint:cyclop // message router; complexity is inherent in the number of loaded-message types.
func (m *Model) routeLoadedDataMsg(msg tea.Msg) []tea.Cmd {
	switch msg := msg.(type) {
	case baseModelsLoadedMsg:
		m.handleBaseModelsLoaded(msg.Items, msg.Gen)
	case importedModelsLoadedMsg:
		m.handleImportedModelsLoaded(msg.Items, msg.Gen)
	case gpuPoolsLoadedMsg:
		return []tea.Cmd{m.handleGPUPoolsLoaded(msg.Items, msg.Gen)}
	case gpuNodesLoadedMsg:
		m.handleGPUNodesLoaded(msg.Items, msg.Gen)
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
