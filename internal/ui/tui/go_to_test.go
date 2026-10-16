package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// newGoToModel selects one endpoint whose DAC, model, runtime and tenant
// are all loaded, plus an unrelated imported model (so the ImportedModel
// category is loaded and the endpoint's shared model isn't in it).
func newGoToModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	acme := models.Tenant{Name: "Acme", IDs: []string{"ocid1.tenancy.oc1..acme"}}
	m.dataset.Tenants = []models.Tenant{acme}
	m.dataset.BaseModels = []models.BaseModel{{Name: "llama"}}
	m.dataset.ServingRuntimes = []models.ServingRuntime{{Name: "vllm"}}
	m.dataset.ImportedModelMap = map[string][]models.ImportedModel{"Acme": {{BaseModel: models.BaseModel{Name: "amaaaaaa-other"}}}}
	m.dataset.DedicatedAIClusterMap = map[string][]models.DedicatedAICluster{"Acme": {{Name: "dac-1"}}}
	m.dataset.InferenceServiceMap = map[string][]models.InferenceService{
		"dac-1": {{Name: "isvc-1", Namespace: "dac-1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm", Owner: &m.dataset.Tenants[0]}},
	}
	m.category = domain.InferenceService
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.updateColumns()
	m.updateRows(true)
	require.Len(t, m.rawRows, 1)
	return m
}

func TestGoToTargets_InferenceService(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	assert.Equal(t, []domain.Scope{
		{Category: domain.Tenant, Name: "Acme"},
		{Category: domain.BaseModel, Name: "llama"},
		// ImportedModel "llama" is dropped: that category is loaded and has no such row.
		{Category: domain.DedicatedAICluster, Name: "dac-1"},
		{Category: domain.ServingRuntime, Name: "vllm"},
	}, m.goToTargets())
}

// An on-demand endpoint's namespace is its model, not a DAC, so it offers
// no DedicatedAICluster target even while DACs aren't loaded.
func TestGoToTargets_OnDemandHasNoDAC(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.dataset.DedicatedAIClusterMap = nil
	m.dataset.InferenceServiceMap = map[string][]models.InferenceService{
		"llama": {{Name: "isvc-od", Namespace: "llama", Type: models.InferenceServiceOnDemand, Model: "llama", Runtime: "vllm"}},
	}
	m.updateRows(true)
	require.Len(t, m.rawRows, 1)
	for _, s := range m.goToTargets() {
		assert.NotEqual(t, domain.DedicatedAICluster, s.Category, "on-demand endpoint offered DAC %q", s.Name)
	}
}

// A target in a category that hasn't loaded yet is kept: jumping loads it.
func TestGoToTargets_KeepsUnloadedCategories(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.dataset.ServingRuntimes = nil
	assert.Contains(t, m.goToTargets(), domain.Scope{Category: domain.ServingRuntime, Name: "vllm"})
}

func TestGoToPicker_NavigatesToChoice(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)

	require.Nil(t, m.openGoTo())
	require.Equal(t, common.RowPickerView, m.viewMode)
	view := m.rowPickerView()
	assert.Contains(t, view, "> Tenant")
	assert.Contains(t, view, "BaseModel           llama")
	assert.Contains(t, view, "[enter] go")

	_, _ = m.updateRowPickerView(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = m.updateRowPickerView(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.BaseModel, m.category)
	assert.Equal(t, &domain.Scope{Category: domain.BaseModel, Name: "llama"}, m.scope)
}

func TestGoToPicker_EscCancels(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	_ = m.openGoTo()
	_, _ = m.updateRowPickerView(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.InferenceService, m.category)
	assert.Empty(t, m.rowPicker.targets)
}

// One target jumps straight there; none is a toast.
func TestOpenGoTo_SingleAndNone(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUWorkloadMap = map[string][]models.GPUWorkload{"node-a": {{Name: "pod-1", Node: "node-a"}}}
	m.category = domain.GPUWorkload
	m.updateColumns()
	m.updateRows(true)
	require.Len(t, m.rawRows, 1)

	_ = m.openGoTo()
	assert.Equal(t, domain.GPUNode, m.category, "the node is the only target")
	assert.Equal(t, &domain.Scope{Category: domain.GPUNode, Name: "node-a"}, m.scope)

	m2 := newTestModel(t)
	m2.category = domain.BaseModel
	m2.rawRows = []table.Row{{"llama"}}
	m2.table.SetRows(m2.rawRows)
	require.NotNil(t, m2.openGoTo())
	assert.Equal(t, "no related rows to go to", m2.toasts.active.msg)
}

// g is advertised wherever o (Parent) is.
func TestGoToShortcut_OfferedWithParent(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		km := keys.ResolveKeys(c, common.ListView)
		assert.Equalf(t, contextHasParent(km), contextHas(km, keys.GoTo), "%s: g and o should be offered together", c)
	}
}

func contextHas(km keys.KeyMap, b key.Binding) bool {
	for _, c := range km.Context {
		if c.Help().Desc == b.Help().Desc {
			return true
		}
	}
	return false
}

// Every picker line has the same width, so the centered block aligns.
func TestRowPickerView_LinesEqualWidth(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	_ = m.openGoTo()
	lines := strings.Split(m.rowPickerView(), "\n")
	for _, l := range lines {
		assert.Equal(t, lipgloss.Width(lines[0]), lipgloss.Width(l), "%q", l)
	}
}
