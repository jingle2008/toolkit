package tui

import (
	"errors"
	"reflect"
	"testing"

	"context"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/testutil"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/require"
)

type fakeLoader struct {
	dataset *models.Dataset
	err     error
}

func (f fakeLoader) LoadDataset(ctx context.Context, repoPath string, env models.Environment) (*models.Dataset, error) {
	return f.dataset, f.err
}
func (f fakeLoader) LoadBaseModels(ctx context.Context, repoPath string, env models.Environment) (map[string]*models.BaseModel, error) {
	return map[string]*models.BaseModel{}, nil
}
func (f fakeLoader) LoadGpuPools(ctx context.Context, repoPath string, env models.Environment) ([]models.GpuPool, error) {
	return nil, nil
}
func (f fakeLoader) LoadGpuNodes(ctx context.Context, repoPath string, env models.Environment) (map[string][]models.GpuNode, error) {
	return map[string][]models.GpuNode{}, nil
}
func (f fakeLoader) LoadDedicatedAIClusters(ctx context.Context, repoPath string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return map[string][]models.DedicatedAICluster{}, nil
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
		{
			name:     "success-init",
			loader:   fakeLoader{dataset: wantDataset},
			init:     true,
			wantData: wantDataset,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m, err := NewModel(
				WithRepoPath("repo"),
				WithEnvironment(models.Environment{Type: "t", Region: "r", Realm: "rl"}),
				WithLoader(tc.loader),
			)
			if err != nil {
				t.Fatalf("NewModel failed: %v", err)
			}
			var msg interface{}
			if tc.init {
				msg = m.Init()()
			} else {
				msg = m.loadData()()
			}
			switch {
			case tc.wantData != nil:
				data, ok := msg.(DataMsg)
				if !ok {
					t.Fatalf("expected DataMsg, got %T", msg)
				}
				if !reflect.DeepEqual(data.Data, tc.wantData) {
					t.Errorf("DataMsg.Data = %v, want %v", data.Data, tc.wantData)
				}
			case tc.wantError != nil:
				emsg, ok := msg.(ErrMsg)
				if !ok {
					t.Fatalf("expected ErrMsg, got %T", msg)
				}
				if !errors.Is(emsg.Err, tc.wantError) {
					t.Errorf("ErrMsg.Err = %v, want %v", emsg.Err, tc.wantError)
				}
			default:
				t.Fatalf("invalid test case: no wantData or wantError")
			}
		})
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
	)
	require.NoError(t, err)
	m.dataset = &models.Dataset{
		Tenants: []models.Tenant{
			{Name: "tenant1", IDs: []string{"id1"}, LimitOverrides: 1, ConsolePropertyOverrides: 2, PropertyOverrides: 3},
		},
		Environments: []models.Environment{
			{Realm: "realm", Type: "type", Region: "region"},
		},
	}
	m.viewWidth = 80
	m.viewHeight = 24
	m.contextCtx = context.Background()
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
	m.enterEditMode(Filter)
	require.Equal(t, Edit, m.mode)
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
	m.enterEditMode(Alias)
	require.Equal(t, Edit, m.mode)
	m.exitEditMode(true)
	require.Equal(t, Normal, m.mode)
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
	m.Update(ErrMsg{Err: nil})
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
	m.Update(FilterMsg{Text: "tenant1"})
	// Simulate errMsg
	m.Update(ErrMsg{Err: nil})
}

func TestCenterTextReturnsCenteredText(t *testing.T) {
	t.Parallel()
	result := centerText("hello", 10, 3)
	testutil.Contains(t, result, "hello")
	testutil.GreaterOrEqual(t, len(result), 10)
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
	)
	require.NoError(t, err)
	testutil.NotNil(t, m)
	testutil.Equal(t, "/repo", m.repoPath)
	testutil.Equal(t, "/kube", m.kubeConfig)
	testutil.Equal(t, env, m.environment)
	testutil.Equal(t, domain.Tenant, m.category)
	testutil.NotNil(t, m.table)
	testutil.NotNil(t, m.textInput)
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
	)
	require.NoError(t, err)
	// Set context.Category to Tenant, m.category to LimitTenancyOverride
	m.context = &domain.ToolkitContext{Name: "scopeA", Category: domain.Tenant}
	m.chosen = false
	cs := m.contextString()
	testutil.Contains(t, cs, "LimitTenancyOverride")
	testutil.Contains(t, cs, "scopeA")

	info := m.infoView()
	testutil.Contains(t, info, "Realm:")
	testutil.Contains(t, info, "Type:")
	testutil.Contains(t, info, "Region:")
}

func TestModel_DetailView_and_ExitDetailView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.enterDetailView()
	// Simulate "esc" key in detail view
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")}
	_, _ = updateDetailView(msg, m)
	m.exitDetailView()
}

// --- Test updateListView and edit mode transitions ---
func TestModel_UpdateListView_Branches(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.mode = Normal
	// Simulate NextCategory and PrevCategory keys
	m.keys.NextCategory = m.keys.Quit
	m.keys.PrevCategory = m.keys.Quit
	keyStr := ""
	if len(m.keys.Quit.Keys()) > 0 {
		keyStr = m.keys.Quit.Keys()[0]
	}
	nextMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	updateListView(nextMsg, m)
	updateListView(nextMsg, m)

	// Simulate FilterItems, JumpTo, ViewDetails, ApplyContext
	m.keys.FilterItems = m.keys.Quit
	m.keys.JumpTo = m.keys.Quit
	m.keys.ViewDetails = m.keys.Quit
	m.keys.ApplyContext = m.keys.Quit
	updateListView(nextMsg, m)
	updateListView(nextMsg, m)
	updateListView(nextMsg, m)
	updateListView(nextMsg, m)

	// Switch to Edit mode and test "enter" and "esc"
	m.mode = Edit
	m.target = Alias
	enterMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")}
	escMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")}
	updateListView(enterMsg, m)
	updateListView(escMsg, m)
	m.target = Filter
	updateListView(enterMsg, m)
	updateListView(escMsg, m)
}

// --- Added: Test for getCurrentItem and handleAdditionalKeys ---

func TestModel_GetCurrentItem_and_HandleAdditionalKeys(t *testing.T) {
	t.Parallel()
	// Setup a Model with a BaseModel in the dataset and table
	bm := &models.BaseModel{Name: "bm1", Version: "v1", Type: "typeA"}
	ds := &models.Dataset{
		BaseModelMap: map[string]*models.BaseModel{
			"bm1": bm,
		},
	}
	// Table row for BaseModel: [Name, Version, Type]
	tbl := table.New()
	tbl.SetColumns([]table.Column{
		{Title: "Name", Width: 10},
		{Title: "Version", Width: 10},
		{Title: "Type", Width: 10},
	})
	tbl.SetRows([]table.Row{{"bm1", "v1", "typeA"}})
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
	)
	require.NoError(t, err)
	m.dataset = ds
	m.category = domain.BaseModel

	// getCurrentItem should return the pointer to bm
	got := m.getCurrentItem()
	require.Equal(t, bm, got)

	// handleAdditionalKeys: cover the ViewModelArtifacts branch
	// Set category to BaseModel and call with a matching key
	m.category = domain.BaseModel
	m.keys.ViewModelArtifacts = m.keys.Quit // Use any key that matches
	m.contextCtx = context.Background()
	keyStr := ""
	if len(m.keys.Quit.Keys()) > 0 {
		keyStr = m.keys.Quit.Keys()[0]
	}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	m.handleAdditionalKeys(msg)
}
