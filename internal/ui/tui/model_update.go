/*
Package tui implements the update/reduce logic for the Model.
*/
package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
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
		data interface{}
		err  error
	)
	switch r.category {
	case domain.BaseModel:
		data, err = r.model.loader.LoadBaseModels(context.Background(), r.model.repoPath, r.model.environment)
	case domain.GpuPool:
		data, err = r.model.loader.LoadGpuPools(context.Background(), r.model.repoPath, r.model.environment)
	case domain.GpuNode:
		data, err = r.model.loader.LoadGpuNodes(context.Background(), r.model.kubeConfig, r.model.environment)
	case domain.DedicatedAICluster:
		data, err = r.model.loader.LoadDedicatedAIClusters(context.Background(), r.model.kubeConfig, r.model.environment)
	}
	if err != nil {
		return ErrMsg{Err: err}
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
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.reLayout = true
		m.updateLayout(msg.Width, msg.Height)
	}
	if !m.chosen {
		return updateListView(msg, m)
	}
	return updateDetailView(msg, m)
}

func (m *Model) enterEditMode(target EditTarget) {
	m.table.Blur()
	m.mode = Edit
	m.target = target
	m.textInput.Focus()

	// Provide category suggestions using domain.ParseCategory aliases.
	keys := []string{
		"t", "tenant",
		"ld", "limitdefinition",
		"cpd", "consolepropertydefinition",
		"pd", "propertydefinition",
		"lto", "limittenancyoverride",
		"cpto", "consolepropertytenancyoverride",
		"pto", "propertytenancyoverride",
		"cpro", "consolepropertyregionaloverride",
		"pro", "propertyregionaloverride",
		"bm", "basemodel",
		"ma", "modelartifact",
		"e", "environment",
		"st", "servicetenancy",
		"gp", "gpupool",
		"gn", "gpunode",
		"dac", "dedicatedaicluster",
	}
	if target == Alias {
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
	if m.target == Alias || resetInput {
		m.textInput.SetSuggestions([]string{})
		m.textInput.ShowSuggestions = false
	}

	m.mode = Normal
	m.target = None
	if resetInput {
		m.textInput.Reset()
	}
	m.textInput.Blur()
	m.table.Focus()
}
