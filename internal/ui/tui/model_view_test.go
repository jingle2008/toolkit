package tui

import (
	"errors"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestModel_updateContent_and_View(t *testing.T) {
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
	m.viewMode = common.DetailsView
	m.selectedKey = "dev-UNKNOWN"
	cmd := m.updateContentAsync()
	require.NotNil(t, cmd)
	msg, ok := cmd().(detailContentRenderedMsg)
	require.True(t, ok)
	m.handleDetailContentRenderedMsg(msg)
	viewStr := m.render()
	assert.IsType(t, "", viewStr)
}

func makeTestModel() *Model {
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	m.viewWidth = 80
	m.viewHeight = 24
	// Set at least as many columns as the row length to avoid panic
	m.table.SetColumns([]table.Column{{Title: "col1"}, {Title: "col2"}})
	m.table.SetRows([]table.Row{{"foo", "bar"}})
	return m
}

func TestInfoView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	out := m.infoView()
	assert.Contains(t, out, "Realm:")
	assert.Contains(t, out, "oc1")
	assert.Contains(t, out, "K8s:")
	assert.Equal(t, 6, lipgloss.Height(out))
}

func TestHandleKubeVersionMsg(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	assert.Contains(t, m.infoView(), "…", "placeholder before fetch")

	m.handleKubeVersionMsg(kubeVersionMsg{Context: "stale-ctx", Version: "v1.0.0"})
	assert.Empty(t, m.k8sVersion, "stale context must be ignored")

	m.handleKubeVersionMsg(kubeVersionMsg{Context: m.environment.KubeContext(), Version: "v1.30.1"})
	assert.Equal(t, "v1.30.1", m.k8sVersion)
	assert.Contains(t, m.infoView(), "v1.30.1")
}

func TestContextString(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.category = domain.Tenant
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "foo"}
	m.selectedKey = "foo"
	// Should only contain "foo" in DetailsView
	m.viewMode = common.DetailsView
	out := m.contextString()
	assert.Contains(t, out, "foo")
}

func TestTruncateString(t *testing.T) {
	t.Parallel()
	s := "hello world"
	assert.Equal(t, s, truncateString(s, 20))
	assert.Contains(t, truncateString("longstring", 5), "…")
}

func TestStatusView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.theme.Context = lipgloss.NewStyle()
	m.theme.Stats = lipgloss.NewStyle()
	m.theme.StatusText = lipgloss.NewStyle()
	m.textInput.SetValue("input")
	out := m.statusView()
	assert.Contains(t, out, "input")
}

func TestUpdateContent_DetailsView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.DetailsView
	m.selectedKey = "foo"
	m.dataset = &models.Dataset{}
	m.viewport.SetContent("")
	m.renderer = &testRenderer{}
	cmd := m.updateContentAsync()
	require.NotNil(t, cmd)
	msg, ok := cmd().(detailContentRenderedMsg)
	require.True(t, ok)
	m.handleDetailContentRenderedMsg(msg)
	assert.NotEmpty(t, m.viewport.View())
}

type testRenderer struct{}

func (testRenderer) RenderJSON(_ any, _ int) (string, error) {
	return "rendered", nil
}

func TestUpdateContent_Error(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.DetailsView
	m.selectedKey = "foo"
	m.dataset = &models.Dataset{}
	m.viewport.SetContent("")
	m.renderer = &errRenderer{}
	cmd := m.updateContentAsync()
	require.NotNil(t, cmd)
	msg, ok := cmd().(detailContentRenderedMsg)
	require.True(t, ok)
	toastCmd := m.handleDetailContentRenderedMsg(msg)
	require.NotNil(t, toastCmd, "render error should emit a toast cmd")
	require.NotNil(t, m.toasts.active)
	assert.Contains(t, m.toasts.active.msg, "render failed")
	assert.Equal(t, toastError, m.toasts.active.sev)
}

type errRenderer struct{}

func (errRenderer) RenderJSON(_ any, _ int) (string, error) {
	return "", errors.New("fail")
}

func TestView_LoadingView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = nil // full-screen LoadingView is only used while the first dataset is still loading
	m.viewMode = common.LoadingView
	out := m.render()
	assert.Contains(t, out, "Loading data")
}

func TestStatusView_LoadingNugget(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.theme.Stats = lipgloss.NewStyle()
	m.pendingTasks = 1
	out := m.statusView()
	// The stopwatch renders elapsed time; pendingTasks>0 means the
	// inline loading cell should appear with a duration token like "0s".
	assert.Contains(t, out, "s")
	// Sanity-check that pendingTasks=0 omits the nugget.
	m.pendingTasks = 0
	bareOut := m.statusView()
	assert.NotEqual(t, out, bareOut)
}

func TestView_HelpView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.HelpView
	out := ansi.Strip(m.render())
	assert.Contains(t, out, "General Actions")
}

func TestView_ListView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.ListView
	out := m.render()
	assert.NotEmpty(t, out)
}

func TestView_DetailsView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.DetailsView
	m.selectedKey = "foo"
	m.viewport.SetContent("details")
	out := m.render()
	assert.Contains(t, out, "details")
}
