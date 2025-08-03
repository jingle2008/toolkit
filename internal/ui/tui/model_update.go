/*
Package tui implements the update/reduce logic for the Model.
*/
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

/*
Update implements the tea.Model interface and updates the Model state in response to a message.
*/
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)
	default:
		return m.delegateToActiveView(msg)
	}
}

func (m *Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.reLayout = true
	m.updateLayout(msg.Width, msg.Height)
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
	case common.ErrorView:
		return m.updateErrorView(msg)
	case common.ExportView:
		return m.updateExportView(msg)
	}
	return m, nil
}
