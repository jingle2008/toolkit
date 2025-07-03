/*
Package tui implements the update/reduce logic for the Model.
*/
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

/*
loadRequest is a command for loading category data using the model's context.
*/
type loadRequest struct {
	category domain.Category
	model    *Model
}

func (r loadRequest) Run() tea.Msg {
	var (
		data any
		err  error
	)
	switch r.category { //nolint:exhaustive
	case domain.BaseModel:
		data, err = r.model.loader.LoadBaseModels(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.GpuPool:
		data, err = r.model.loader.LoadGpuPools(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.GpuNode:
		data, err = r.model.loader.LoadGpuNodes(r.model.ctx, r.model.kubeConfig, r.model.environment)
	case domain.DedicatedAICluster:
		data, err = r.model.loader.LoadDedicatedAIClusters(r.model.ctx, r.model.kubeConfig, r.model.environment)
	case domain.Tenant, domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride:
		data, err = r.model.loader.LoadTenancyOverrideGroup(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.LimitRegionalOverride:
		data, err = r.model.loader.LoadLimitRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.ConsolePropertyRegionalOverride:
		data, err = r.model.loader.LoadConsolePropertyRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	case domain.PropertyRegionalOverride:
		data, err = r.model.loader.LoadPropertyRegionalOverrides(r.model.ctx, r.model.repoPath, r.model.environment)
	}
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to load %s: %w", r.category, err))
	}
	return DataMsg{Data: data}
}

/*
Update implements the tea.Model interface and updates the Model state in response to a message.
*/
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.reduce(msg)
}

// reduce is a pure state reducer for Model, used for testability.
func (m *Model) reduce(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.reLayout = true
		m.updateLayout(msg.Width, msg.Height)
	}

	switch m.viewMode { //nolint:exhaustive
	case common.HelpView:
		return m.updateHelpView(msg)
	case common.ListView:
		return m.updateListView(msg)
	case common.DetailsView:
		return m.updateDetailView(msg)
	}
	return m, nil
}

func (m *Model) enterEditMode(target common.EditTarget) {
	m.table.Blur()
	m.inputMode = common.EditInput
	m.editTarget = target
	m.textInput.Focus()

	// Provide category suggestions using domain.Aliases().
	keys := domain.Aliases()
	if target == common.AliasTarget {
		m.textInput.Reset()
	} else if len(m.textInput.Value()) > 0 {
		keys = append(keys, m.textInput.Value())
		m.backToLastState()
	}

	m.textInput.ShowSuggestions = len(keys) > 0
	m.textInput.SetSuggestions(keys)
}

func (m *Model) backToLastState() {
	if m.curFilter != "" {
		m.textInput.Reset()
		FilterTable(m, "")
	} else if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		m.context = nil
		m.updateRows()
	}
}

func (m *Model) exitEditMode(resetInput bool) {
	if m.editTarget == common.AliasTarget || resetInput {
		m.textInput.SetSuggestions([]string{})
		m.textInput.ShowSuggestions = false
	}

	m.inputMode = common.NormalInput
	m.editTarget = common.NoneTarget
	if resetInput {
		m.textInput.Reset()
	}
	m.textInput.Blur()
	m.table.Focus()
}
