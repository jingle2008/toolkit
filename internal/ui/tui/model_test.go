package tui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLoader struct {
	dataset *models.Dataset
	err     error
}

func (f fakeLoader) LoadDataset(_ context.Context, _ string, _ models.Environment) (*models.Dataset, error) {
	return f.dataset, f.err
}

func (f fakeLoader) LoadBaseModels(_ context.Context, _ string, _ models.Environment) (map[string]*models.BaseModel, error) {
	return map[string]*models.BaseModel{}, nil
}

func (f fakeLoader) LoadGpuPools(_ context.Context, _ string, _ models.Environment) ([]models.GpuPool, error) {
	return nil, nil
}

func (f fakeLoader) LoadGpuNodes(_ context.Context, _ string, _ models.Environment) (map[string][]models.GpuNode, error) {
	return map[string][]models.GpuNode{}, nil
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
				msg = m.loadData()[1]()
			}
			checkLoadDataResult(t, msg, tc.wantData, tc.wantError)
		})
	}
}

func checkLoadDataResult(t *testing.T, msg any, wantData *models.Dataset, wantError error) {
	t.Helper()
	switch {
	case wantData != nil:
		data, ok := msg.(DataMsg)
		if !ok {
			t.Fatalf("expected DataMsg, got %T", msg)
		}
		if !reflect.DeepEqual(data.Data, wantData) {
			t.Errorf("DataMsg.Data = %v, want %v", data.Data, wantData)
		}
	case wantError != nil:
		emsg, ok := msg.(ErrMsg)
		if !ok {
			t.Fatalf("expected ErrMsg, got %T", msg)
		}
		if !errors.Is(emsg, wantError) {
			t.Errorf("ErrMsg = %v, want %v", emsg, wantError)
		}
	default:
		t.Fatalf("invalid test case: no wantData or wantError")
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
	m.enterEditMode(common.FilterTarget)
	require.Equal(t, common.EditInput, m.inputMode)
	m.textInput.SetValue("tenant1")
	cmd := DebounceFilter(m)
	require.NotNil(t, cmd)
	// Simulate filterMsg
	FilterTable(m, "tenant1")
	require.Equal(t, "tenant1", m.curFilter)
	m.backToLastState()
	require.Equal(t, "", m.curFilter)
}

func TestEditModeTransitions(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.enterEditMode(common.AliasTarget)
	require.Equal(t, common.EditInput, m.inputMode)
	m.exitEditMode(true)
	require.Equal(t, common.NormalInput, m.inputMode)
}

func TestProcessDataAndErrorMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// processData with *models.Dataset
	m.processData(DataMsg{Data: m.dataset})
	// processData with map[string]*models.BaseModel
	m.processData(DataMsg{Data: map[string]*models.BaseModel{"bm": {}}})
	// processData with []models.GpuPool
	m.processData(DataMsg{Data: []models.GpuPool{{}}})
	// processData with map[string][]models.GpuNode
	m.processData(DataMsg{Data: map[string][]models.GpuNode{"pool": {}}})
	// processData with map[string][]models.DedicatedAICluster
	m.processData(DataMsg{Data: map[string][]models.DedicatedAICluster{"tenant": {}}})
	// Update with errorMsg
	m.Update(ErrMsg(nil))
}

func TestModelUpdateBranches(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Simulate tea.KeyMsg for "ctrl+c"
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// Simulate tea.WindowSizeMsg
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	// Simulate dataMsg
	m.Update(DataMsg{Data: m.dataset})
	// Simulate filterMsg
	m.Update(FilterMsg("tenant1"))
	// Simulate errMsg
	m.Update(ErrMsg(nil))
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
	// Set context.Category to Tenant, m.category to LimitTenancyOverride
	m.context = &domain.ToolkitContext{Name: "scopeA", Category: domain.Tenant}
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
	if len(keys.FilterList.Keys()) > 0 {
		keyStr = keys.FilterList.Keys()[0]
	}
	filterMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.updateListView(filterMsg)

	// Simulate JumpTo key
	if len(keys.JumpTo.Keys()) > 0 {
		keyStr = keys.JumpTo.Keys()[0]
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
	bm := &models.BaseModel{InternalName: "v1", Name: "bm1", Version: "v1", Type: "typeA"}
	ds := &models.Dataset{
		BaseModelMap: map[string]*models.BaseModel{
			"bm1": bm,
		},
	}
	// Table row for BaseModel: [InternalName, Name, Version, Type]
	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "InternalName", Width: 10},
		{Title: "Name", Width: 10},
		{Title: "Version", Width: 10},
		{Title: "Type", Width: 10},
	})
	tbl.SetRows([]table.Row{{"v1", "bm1", "v1", "typeA"}})
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
	got := m.getSelectedItem()
	require.Equal(t, bm, got)
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
		BaseModelMap:          map[string]*models.BaseModel{},
		GpuPools:              []models.GpuPool{},
		GpuNodeMap:            map[string][]models.GpuNode{},
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
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.table.SetColumns([]table.Column{{Title: "Region", Width: 10}})
	m.table.SetRows([]table.Row{{"dev-UNKNOWN"}})
	m.category = domain.Environment
	m.dataset = &models.Dataset{
		Environments: []models.Environment{
			{Type: "dev", Region: "us-phx-1", Realm: "oc1"},
		},
	}
	cmd := m.enterContext()
	// It is valid for cmd to be nil if no update is needed; just ensure no panic
	_ = cmd
}
