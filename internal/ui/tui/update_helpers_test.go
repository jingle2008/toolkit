package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

type dummyLogger struct{}

func (dummyLogger) Errorw(string, ...any)            {}
func (dummyLogger) Debugw(string, ...any)            {}
func (dummyLogger) Infow(string, ...any)             {}
func (dummyLogger) Warnw(string, ...any)             {}
func (dummyLogger) Sync() error                      { return nil }
func (dummyLogger) WithFields(...any) logging.Logger { return dummyLogger{} }
func (dummyLogger) DebugEnabled() bool               { return false }

type dummyLoader struct{}

var errDummy = errors.New("dummy loader: not implemented")

func (dummyLoader) LoadDataset(_ context.Context, _ string, _ models.Environment) (*models.Dataset, error) {
	return nil, errDummy
}

func (dummyLoader) LoadBaseModels(_ context.Context, _ string, _ models.Environment) ([]models.BaseModel, error) {
	return nil, errDummy
}

func (dummyLoader) LoadImportedModels(_ context.Context, _ string, _ models.Environment) (map[string][]models.ImportedModel, error) {
	return nil, errDummy
}

func (dummyLoader) LoadGPUPools(_ context.Context, _ string, _ models.Environment) ([]models.GPUPool, error) {
	return nil, errDummy
}

func (dummyLoader) LoadGPUNodesByPool(_ context.Context, _ string, _ models.Environment) (map[string][]models.GPUNode, error) {
	return nil, errDummy
}

func (dummyLoader) LoadGPUWorkloadsByNode(_ context.Context, _ string, _ models.Environment) (map[string][]models.GPUWorkload, error) {
	return nil, errDummy
}

func (dummyLoader) LoadDedicatedAIClusters(_ context.Context, _ string, _ models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return nil, errDummy
}

func (dummyLoader) LoadTenancyOverrideGroup(_ context.Context, _ string, _ models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{}, errDummy
}

func (dummyLoader) LoadLimitRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.LimitRegionalOverride, error) {
	return nil, errDummy
}

func (dummyLoader) LoadConsolePropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return nil, errDummy
}

func (dummyLoader) LoadPropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.PropertyRegionalOverride, error) {
	return nil, errDummy
}

func Test_updateLoadingView_QuitKey(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.viewMode = common.LoadingView
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.updateLoadingView(msg)
	require.NotNil(t, cmd)
	// tea.Quit is a function returning tea.QuitMsg, so call cmd() and check type
	res := cmd()
	require.IsType(t, tea.QuitMsg{}, res)
}

func Test_Update_SpinnerTick(t *testing.T) {
	t.Parallel()
	m := &Model{pendingTasks: 1}
	m.viewMode = common.LoadingView
	s := spinner.New()
	r := stopwatch.New()
	m.loadingSpinner = &s
	m.loadingTimer = &r
	_, cmd := m.Update(spinner.TickMsg{})
	require.NotNil(t, cmd)
}

func Test_Update_DataMsgWithEmptyData(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.LoadingView
	m.dataset = &models.Dataset{}
	_, cmd := m.Update(dataMsg{})
	require.Nil(t, cmd)
}

func Test_Update_ErrMsgEmitsToast(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.LoadingView
	_, cmd := m.Update(errMsg{err: errors.New("fail")})
	require.NotNil(t, cmd, "errMsg should return a tea.Cmd (toast auto-dismiss tick)")
	require.NotNil(t, m.toast, "errMsg should set an error toast")
	require.Equal(t, "fail", m.toast.msg)
}

func Test_updateHelpView_KeyMsg(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.HelpView
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.updateHelpView(msg)
	require.NotNil(t, cmd)
	res := cmd()
	require.IsType(t, tea.QuitMsg{}, res)
}

func Test_updateHelpView_OtherMsg(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.HelpView
	msg := dataMsg{}
	_, cmd := m.updateHelpView(msg)
	require.Nil(t, cmd)
}

func Test_updateDetailView_NoSelectedRow(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.DetailsView
	// No selected row
	msg := dataMsg{}
	_, cmd := m.updateDetailView(msg)
	require.Nil(t, cmd)
}

func Test_updateDetailView_WithSelectedRow(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.DetailsView
	// Simulate a selected row
	m.table.SetColumns([]table.Column{
		{Title: "ID", Width: 10},
		{Title: "Value", Width: 10},
	})
	m.table.SetRows([]table.Row{{"id", "value"}})
	m.table.SetCursor(0)
	msg := dataMsg{}
	_, cmd := m.updateDetailView(msg)
	require.Nil(t, cmd)
}
