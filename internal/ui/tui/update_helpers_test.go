package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/require"
)

type dummyLogger struct{}

func (dummyLogger) Errorw(string, ...any)            {}
func (dummyLogger) Debugw(string, ...any)            {}
func (dummyLogger) Infow(string, ...any)             {}
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

func (dummyLoader) LoadGpuPools(_ context.Context, _ string, _ models.Environment) ([]models.GpuPool, error) {
	return nil, errDummy
}

func (dummyLoader) LoadGpuNodes(_ context.Context, _ string, _ models.Environment) (map[string][]models.GpuNode, error) {
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

func Test_updateLoadingView_SpinnerTick(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.viewMode = common.LoadingView
	s := spinner.New()
	r := stopwatch.New()
	m.loadingSpinner = &s
	m.loadingTimer = &r
	msg := spinner.TickMsg{}
	_, cmd := m.updateLoadingView(msg)
	require.NotNil(t, cmd)
}

func Test_updateLoadingView_DataMsg(t *testing.T) {
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
	msg := DataMsg{}
	_, cmd := m.updateLoadingView(msg)
	require.Nil(t, cmd)
}

func Test_updateLoadingView_ErrMsg(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("/tmp"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "e"}),
		WithLoader(dummyLoader{}),
		WithLogger(dummyLogger{}),
	)
	require.NoError(t, err)
	m.viewMode = common.LoadingView
	msg := ErrMsg(errors.New("fail"))
	_, cmd := m.updateLoadingView(msg)
	require.Nil(t, cmd)
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
	msg := DataMsg{}
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
	msg := DataMsg{}
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
	msg := DataMsg{}
	_, cmd := m.updateDetailView(msg)
	require.Nil(t, cmd)
}
