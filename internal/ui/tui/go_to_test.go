package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	acme := models.Tenant{Name: "Acme", ID: "ocid1.tenancy.oc1..acme"}
	m.dataset.Tenants = []models.Tenant{acme}
	m.dataset.BaseModels = []models.BaseModel{{Name: "llama"}}
	m.dataset.ServingRuntimes = []models.ServingRuntime{{Name: "vllm"}}
	m.dataset.ImportedModelMap = map[string][]models.ImportedModel{"Acme": {{BaseModel: models.BaseModel{Name: "amaaaaaa-other"}}}}
	m.dataset.DedicatedAIClusterMap = map[string][]models.DedicatedAICluster{"Acme": {{Name: "amaaaaaa-dac1"}}}
	m.dataset.InferenceServiceMap = map[string][]models.InferenceService{
		"amaaaaaa-dac1": {{Name: "isvc-1", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm", Owner: &m.dataset.Tenants[0]}},
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
		{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"},
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
	view := ansi.Strip(m.rowPickerView())
	assert.Contains(t, lineWith(view, "DedicatedAICluster"), "▸", "starts on the structural parent")
	assert.Contains(t, lineWith(view, "BaseModel"), "llama")
	assert.NotContains(t, lineWith(view, "BaseModel"), "▸")
	assert.Contains(t, view, "enter go")

	_, _ = m.updateRowPickerView(keyPress("up"))
	_, _ = m.updateRowPickerView(keyPress("enter"))
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.BaseModel, m.category)
	assert.Equal(t, &domain.Scope{Category: domain.BaseModel, Name: "llama"}, m.scope)
}

func TestGoToPicker_EscCancels(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	_ = m.openGoTo()
	_, _ = m.updateRowPickerView(keyPress("esc"))
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

// o is offered in list view on every category with a parent, never in
// details view (where o is Copy Object), and not on parentless ones.
func TestGoToShortcut_OfferedInSubCategoriesOnly(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories() {
		_, hasRelated := rowSources[c]
		hasRelated = hasRelated && rowSources[c].related != nil && len(rowSources[c].parents) > 0
		assert.Equalf(t, hasRelated, contextHas(keys.ResolveKeys(c, common.ListView), keys.GoTo),
			"%s: o offered iff the category has relations", c)
		assert.Falsef(t, contextHas(keys.ResolveKeys(c, common.DetailsView), keys.GoTo),
			"%s: o is Copy Object in details view", c)
	}
}

// o reaches openGoTo through Update, and the table under the picker keeps
// its cursor: when go-to was on g, the table's own g (go to top) also
// fired, so esc returned to the first row instead of the selected one.
func TestGoToKey_PickerKeepsListCursor(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	owner := &m.dataset.Tenants[0]
	m.dataset.InferenceServiceMap["amaaaaaa-dac1"] = append(m.dataset.InferenceServiceMap["amaaaaaa-dac1"],
		models.InferenceService{Name: "isvc-2", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm", Owner: owner},
		models.InferenceService{Name: "isvc-3", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm", Owner: owner})
	m.updateRows(true)
	m.viewMode = common.ListView
	m.table.Focus()
	m.table.SetCursor(2)
	require.Equal(t, "isvc-3", m.selectedRawRow()[0])

	_, _ = m.Update(keyPress("o"))
	require.Equal(t, common.RowPickerView, m.viewMode)
	assert.Equal(t, domain.DedicatedAICluster, m.rowPicker.targets[m.rowPicker.cursor].Category,
		"the picker starts on the endpoint's DAC, its structural parent")

	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, "isvc-3", m.selectedRawRow()[0])
}

func TestGPUWorkloadKeys(t *testing.T) {
	t.Parallel()
	km := keys.ResolveKeys(domain.GPUWorkload, common.ListView)
	assert.True(t, contextHas(km, keys.GoTo), "GPUWorkload list view should offer Go to related")
	assert.True(t, contextHas(km, keys.SortTenant), "GPUWorkload list view should offer Sort Tenant")
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

// Enter on a tenant drills into its first child category with rows:
// Acme has no overrides, so DedicatedAICluster, not LimitTenancyOverride.
func TestDrillTarget_SkipsEmptyChildren(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.category = domain.Tenant
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "Acme"}
	assert.Equal(t, domain.DedicatedAICluster, m.drillTarget())

	m.scope = &domain.Scope{Category: domain.Tenant, Name: "nobody"}
	assert.Equal(t, domain.LimitTenancyOverride, m.drillTarget(), "with no rows anywhere, the first child")
}

// esc in a drilled-in list returns to the parent, on the row drilled in
// from, by stepping back in history, so ] leads into the child again.
func TestEsc_ScopedListReturnsToParent(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	m.updateCategory(domain.Tenant)
	m.updateRows(true)
	m.history, m.historyIdx = histOf(domain.Tenant), 0

	_, _ = m.Update(keyPress("enter"))
	require.Equal(t, domain.DedicatedAICluster, m.category)
	require.Equal(t, []domain.Category{domain.Tenant, domain.DedicatedAICluster}, histCategories(m))

	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, domain.Tenant, m.category)
	assert.Equal(t, 0, m.historyIdx, "back in history, not pushed")
	m.updateRows(true)
	assert.Equal(t, "Acme", m.selectedRawRow()[0], "the cursor lands on the row drilled in from")

	_, _ = m.Update(keyPress("]"))
	assert.Equal(t, domain.DedicatedAICluster, m.category)
	assert.Contains(t, m.contextString(), "Acme", "forward returns to the scoped child")
}

// With the parent not in history (reached by tab, say), esc pushes it.
func TestEsc_ScopedListPushesParent(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	m.scope = &domain.Scope{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"}
	m.history, m.historyIdx = histOf(domain.InferenceService), 0

	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, domain.DedicatedAICluster, m.category)
	assert.Equal(t, []domain.Category{domain.InferenceService, domain.DedicatedAICluster}, histCategories(m))
}
