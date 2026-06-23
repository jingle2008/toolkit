package tui

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

type fakeLoader struct {
	dataset *models.Dataset
	err     error
}

func (f fakeLoader) LoadDataset(_ context.Context, _ string, _ models.Environment) (*models.Dataset, error) {
	return f.dataset, f.err
}

func (f fakeLoader) LoadBaseModels(_ context.Context, _ string, _ models.Environment) ([]models.BaseModel, error) {
	return []models.BaseModel{}, nil
}

func (f fakeLoader) LoadImportedModels(_ context.Context, _ string, _ models.Environment) (map[string][]models.ImportedModel, error) {
	return map[string][]models.ImportedModel{}, nil
}

func (f fakeLoader) LoadGPUPools(_ context.Context, _ string, _ models.Environment) ([]models.GPUPool, error) {
	return nil, nil
}

func (f fakeLoader) LoadGPUNodesByPool(_ context.Context, _ string, _ models.Environment) (map[string][]models.GPUNode, error) {
	return map[string][]models.GPUNode{}, nil
}

func (f fakeLoader) LoadGPUWorkloadsByNode(_ context.Context, _ string, _ models.Environment) (map[string][]models.GPUWorkload, error) {
	return map[string][]models.GPUWorkload{}, nil
}

func (f fakeLoader) LoadDedicatedAIClusters(_ context.Context, _ string, _ models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return map[string][]models.DedicatedAICluster{}, nil
}

// TenancyOverrideLoader stubs
func (f fakeLoader) LoadLimitTenancyOverrides(_ context.Context, _ string, _ models.Environment) (map[string][]models.LimitTenancyOverride, error) {
	return map[string][]models.LimitTenancyOverride{}, nil
}

func (f fakeLoader) LoadConsolePropertyTenancyOverrides(_ context.Context, _ string, _ models.Environment) (map[string][]models.ConsolePropertyTenancyOverride, error) {
	return map[string][]models.ConsolePropertyTenancyOverride{}, nil
}

func (f fakeLoader) LoadPropertyTenancyOverrides(_ context.Context, _ string, _ models.Environment) (map[string][]models.PropertyTenancyOverride, error) {
	return map[string][]models.PropertyTenancyOverride{}, nil
}

func (f fakeLoader) LoadTenancyOverrideGroup(_ context.Context, _ string, _ models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{}, nil
}

// RegionalOverrideLoader stubs
func (f fakeLoader) LoadLimitRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.LimitRegionalOverride, error) {
	return []models.LimitRegionalOverride{}, nil
}

func (f fakeLoader) LoadConsolePropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return []models.ConsolePropertyRegionalOverride{}, nil
}

func (f fakeLoader) LoadPropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.PropertyRegionalOverride, error) {
	return []models.PropertyRegionalOverride{}, nil
}

func TestModel_LoadData_TableDriven(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name      string
		loader    fakeLoader
		init      bool
		wantData  *models.Dataset
		wantError error
	}
	wantDataset := &models.Dataset{}
	fakeErr := errors.New("fail")
	tests := []testCase{
		{
			name:     "success-loadData",
			loader:   fakeLoader{dataset: wantDataset},
			init:     false,
			wantData: wantDataset,
		},
		{
			name:      "error-loadData",
			loader:    fakeLoader{err: fakeErr},
			init:      false,
			wantError: fakeErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := NewModel(
				WithRepoPath("repo"),
				WithEnvironment(models.Environment{Type: "t", Region: "r", Realm: "rl"}),
				WithLoader(tc.loader),
				WithLogger(logging.NewNoOpLogger()),
			)
			if err != nil {
				t.Fatalf("NewModel failed: %v", err)
			}
			var msg any
			if tc.init {
				msg = m.Init()()
			} else {
				msg = m.loadData()[2]()
			}
			checkLoadDataResult(t, msg, tc.wantData, tc.wantError)
		})
	}
}

func checkLoadDataResult(t *testing.T, msg any, wantData *models.Dataset, wantError error) {
	t.Helper()
	if wantData != nil {
		assertLoadDataMessage(t, msg, wantData)
		return
	}
	if wantError != nil {
		assertLoadErrorMessage(t, msg, wantError)
		return
	}
	t.Fatalf("invalid test case: no wantData or wantError")
}

func assertLoadDataMessage(t *testing.T, msg any, wantData *models.Dataset) {
	t.Helper()
	switch m := msg.(type) {
	case dataMsg:
		if !reflect.DeepEqual(m.Data, wantData) {
			t.Errorf("dataMsg.Data = %v, want %v", m.Data, wantData)
		}
	case datasetLoadedMsg:
		if !reflect.DeepEqual(m.Dataset, wantData) {
			t.Errorf("datasetLoadedMsg.Dataset = %v, want %v", m.Dataset, wantData)
		}
	default:
		t.Fatalf("expected dataMsg or datasetLoadedMsg, got %T", msg)
	}
}

func assertLoadErrorMessage(t *testing.T, msg any, wantError error) {
	t.Helper()
	emsg, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("expected errMsg, got %T", msg)
	}
	if !errors.Is(emsg, wantError) {
		t.Errorf("errMsg = %v, want %v", emsg, wantError)
	}
}

func newTestModel(t *testing.T) *Model {
	t.Helper()
	env := models.Environment{
		Realm:  "realm",
		Type:   "type",
		Region: "region",
	}
	m, err := NewModel(
		WithRepoPath("testrepo"),
		WithEnvironment(env),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	m.dataset = &models.Dataset{
		Tenants: []models.Tenant{
			{Name: "tenant1", IDs: []string{"id1"}},
		},
		Environments: []models.Environment{
			{Realm: "realm", Type: "type", Region: "region"},
		},
	}
	m.viewWidth = 80
	m.viewHeight = 24
	m.refreshDisplay()
	return m
}

func TestUpdateLayoutAndView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.updateLayout(80, 24)
	require.Equal(t, 80, m.viewWidth)
	require.Equal(t, 24, m.viewHeight)
	_ = m.View()
	m.enterDetailView()
	_ = m.View()
}

func TestContextStringAndStatusView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	s := m.contextString()
	require.Contains(t, s, "all")
	status := m.statusView()
	require.NotEmpty(t, status)
	m.enterDetailView()
	_ = m.contextString()
}

func TestFilterAndBackToLastState(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.enterFilterMode()
	require.Equal(t, common.EditInput, m.inputMode)
	m.textInput.SetValue("tenant1")
	cmd := DebounceFilter(m)
	require.NotNil(t, cmd)
	// Simulate filter apply through async rows computation.
	cmd = filterTableAsync(m, "tenant1")
	require.NotNil(t, cmd)
	rowsMsg, ok := cmd().(tableRowsComputedMsg)
	require.True(t, ok)
	m.handleTableRowsComputedMsg(rowsMsg)
	require.Equal(t, "tenant1", m.filter)
	cmd = m.backToLastState()
	require.NotNil(t, cmd)
	rowsMsg, ok = cmd().(tableRowsComputedMsg)
	require.True(t, ok)
	m.handleTableRowsComputedMsg(rowsMsg)
	require.Equal(t, "", m.filter)
}

func TestEditModeTransitions(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.enterAliasMode()
	require.Equal(t, common.EditInput, m.inputMode)
	m.exitEditMode(true)
	require.Equal(t, common.NormalInput, m.inputMode)
}

func TestProcessDataAndErrorMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// handleDataMsg with the foundational dataset and the nil refresh signal.
	m.handleDataMsg(dataMsg{Data: m.dataset})
	m.handleDataMsg(dataMsg{})
	// Per-category data now flows through the typed handlers.
	m.handleBaseModelsLoaded([]models.BaseModel{{}}, m.gens.msg)
	m.handleGPUPoolsLoaded([]models.GPUPool{{}}, m.gens.msg)
	m.handleGPUNodesLoaded(map[string][]models.GPUNode{"pool": {}}, m.gens.msg)
	m.handleDedicatedAIClustersLoaded(map[string][]models.DedicatedAICluster{"tenant": {}}, m.gens.msg)
	// Update with errorMsg
	m.Update(errMsg{})
}

func TestModelUpdateBranches(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Simulate tea.KeyMsg for "ctrl+c"
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// Simulate tea.WindowSizeMsg
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	// Simulate dataMsg
	m.Update(dataMsg{Data: m.dataset})
	// Simulate filterMsg
	m.Update(filterMsg("tenant1"))
	// Simulate errMsg
	m.Update(errMsg{})
}

func TestCenterTextReturnsCenteredText(t *testing.T) {
	t.Parallel()
	result := view.CenterText("hello", 10, 3)
	assert.Contains(t, result, "hello")
	assert.GreaterOrEqual(t, len(result), 10)
}

func TestNewModelInitializesFields(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m, err := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(domain.Tenant),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "/repo", m.repoPath)
	assert.Equal(t, "/kube", m.kubeConfig)
	assert.Equal(t, env, m.environment)
	assert.Equal(t, domain.Tenant, m.category)
	require.NotNil(t, m.table)
	require.NotNil(t, m.textInput)
}

func TestModelContextStringAndInfoView(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m, err := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(domain.LimitTenancyOverride),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	// Set scope.Category to Tenant, m.category to LimitTenancyOverride
	m.scope = &domain.Scope{Name: "scopeA", Category: domain.Tenant}
	m.viewMode = common.ListView
	cs := m.contextString()
	assert.Contains(t, cs, "LimitTenancyOverride")
	assert.Contains(t, cs, "scopeA")

	info := m.infoView()
	assert.Contains(t, info, "Realm:")
	assert.Contains(t, info, "Type:")
	assert.Contains(t, info, "Region:")
}

func TestModel_DetailView_and_ExitDetailView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.enterDetailView()
	// Simulate "esc" key in detail view
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")}
	_, _ = m.updateDetailView(msg)
	m.exitDetailView()
}

// --- Test updateListView and edit mode transitions ---
func TestModel_UpdateListView_Branches(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.inputMode = common.NormalInput

	// Simulate Quit key
	keyStr := ""
	if len(keys.Quit.Keys()) > 0 {
		keyStr = keys.Quit.Keys()[0]
	}
	quitMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(quitMsg)

	// Simulate NextCategory key
	if len(keys.NextCategory.Keys()) > 0 {
		keyStr = keys.NextCategory.Keys()[0]
	}
	nextCatMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(nextCatMsg)

	// Simulate PrevCategory key
	if len(keys.PrevCategory.Keys()) > 0 {
		keyStr = keys.PrevCategory.Keys()[0]
	}
	prevCatMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(prevCatMsg)

	// Simulate FilterItems key
	if len(keys.FilterMode.Keys()) > 0 {
		keyStr = keys.FilterMode.Keys()[0]
	}
	filterMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(filterMsg)

	// Simulate JumpTo key
	if len(keys.CommandMode.Keys()) > 0 {
		keyStr = keys.CommandMode.Keys()[0]
	}
	jumpToMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(jumpToMsg)

	// Simulate ViewDetails key
	if len(keys.ViewDetails.Keys()) > 0 {
		keyStr = keys.ViewDetails.Keys()[0]
	}
	viewDetailsMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(viewDetailsMsg)

	// Simulate ApplyContext key
	if len(keys.Confirm.Keys()) > 0 {
		keyStr = keys.Confirm.Keys()[0]
	}
	applyContextMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(applyContextMsg)

	// Switch to Edit mode and test "enter" and "esc"
	m.inputMode = common.EditInput
	m.editTarget = common.AliasTarget
	enterMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")}
	escMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")}
	m.updateListView(enterMsg)
	m.updateListView(escMsg)
	m.editTarget = common.FilterTarget
	m.updateListView(enterMsg)
	m.updateListView(escMsg)
}

// --- Added: Test for getCurrentItem and handleAdditionalKeys ---

func TestModel_GetCurrentItem_and_HandleAdditionalKeys(t *testing.T) {
	t.Parallel()
	// Setup a Model with a BaseModel in the dataset and table
	bm := models.BaseModel{InternalName: "v1", Name: "bm1", Version: "v1", Type: "typeA"}
	ds := &models.Dataset{
		BaseModels: []models.BaseModel{bm},
	}
	// Table row for BaseModel: [Name, InternalName, Version, Type]
	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "Name", Width: 10},
		{Title: "InternalName", Width: 10},
		{Title: "Version", Width: 10},
		{Title: "Type", Width: 10},
	})
	tbl.SetRows([]table.Row{{"bm1", "v1", "v1", "typeA"}})
	tbl.SetCursor(0)

	env := models.Environment{
		Realm:  "realm",
		Type:   "type",
		Region: "region",
	}
	m, err := NewModel(
		WithTable(&tbl),
		WithRepoPath("testrepo"),
		WithEnvironment(env),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	m.dataset = ds
	m.category = domain.BaseModel

	// getCurrentItem should return the pointer to bm
	got := m.selectedItem()
	require.Equal(t, &bm, got)
}

// Finding #4 / category-drift guard: every kube-backed category must be in
// lazyLoadedCategories. The base LoadDataset carries no cluster data, so a
// kube-backed category absent from this set never loads on direct
// `toolkit -c <cat>` startup (no navigation event fires to trigger it).
func TestLazyLoadedCategories_CoversKubeBacked(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		if !c.NeedsKubeConfig() {
			continue
		}
		_, ok := lazyLoadedCategories[c]
		assert.Truef(t, ok, "kube-backed category %s must be in lazyLoadedCategories so it loads on direct startup", c)
	}
}

// Finding #6: DAC deletion is a multi-minute workflow with its own internal
// timeout. It must use longOpCtx (no 30s cap), not opCtx, or the parent ctx
// cancels mid-workflow after endpoint deletion succeeds but before the cluster
// delete/polling finishes.
func TestLongOpCtx_NoShortCapAndCancelsOnShutdown(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
		WithContext(parent),
	)
	require.NoError(t, err)

	longCtx := m.longOpCtx()
	_, hasDeadline := longCtx.Deadline()
	assert.False(t, hasDeadline, "long-op ctx must not impose the 30s one-shot deadline")

	// Sanity: opCtx still caps one-shot actions (cordon/drain/scale) at ~30s.
	opCtx, opCancel := m.opCtx()
	defer opCancel()
	dl, ok := opCtx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(30*time.Second), dl, 2*time.Second)

	// But long-op ctx still cancels on app shutdown.
	cancel()
	assert.Error(t, longCtx.Err())
}

func TestModel_Init(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestModel_updateColumns(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.table.SetWidth(80)
	m.category = domain.BaseModel
	m.updateColumns()
	assert.NotEmpty(t, m.headers)
}

func TestModel_updateCategory(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.dataset = &models.Dataset{}
	cmd := m.updateCategory(domain.BaseModel)
	assert.NotNil(t, cmd)
}

func TestModel_changeCategory(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.dataset = &models.Dataset{
		BaseModels:            []models.BaseModel{},
		GPUPools:              []models.GPUPool{},
		GPUNodeMap:            map[string][]models.GPUNode{},
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{},
	}
	ti := textinput.New()
	ti.SetValue("BaseModel")
	m.textInput = &ti
	cmd := m.changeCategory()
	assert.NotNil(t, cmd)
}

func TestModel_enterContext(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	// Region.Code("us-phoenix-1") == "phx", so Environment.GetName() == "dev-phx".
	m.table.SetColumns([]table.Column{{Title: "Region", Width: 10}})
	m.table.SetRows([]table.Row{{"dev-phx"}})
	m.category = domain.Environment
	m.dataset = &models.Dataset{
		Environments: []models.Environment{
			{Type: "dev", Region: "us-phoenix-1", Realm: "oc1"},
		},
	}
	cmd := m.enterContext()
	// It is valid for cmd to be nil if no update is needed; just ensure no panic.
	_ = cmd
}
