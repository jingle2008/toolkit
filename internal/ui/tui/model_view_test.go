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

func TestEnvBar(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	out := ansi.Strip(m.envBar())
	assert.Contains(t, out, "oc1 › dev › us-phx-1")
	assert.Contains(t, out, "k8s")
	assert.Equal(t, 1, lipgloss.Height(out))
	assert.Equal(t, m.viewWidth, lipgloss.Width(out))
}

func TestHandleKubeVersionMsg(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	assert.Contains(t, ansi.Strip(m.envBar()), "k8s …", "placeholder before fetch")

	m.handleKubeVersionMsg(kubeVersionMsg{Context: "stale-ctx", Version: "v1.0.0"})
	assert.Empty(t, m.k8sVersion, "stale context must be ignored")

	m.handleKubeVersionMsg(kubeVersionMsg{Context: m.environment.KubeContext(), Version: "v1.30.1"})
	assert.Equal(t, "v1.30.1", m.k8sVersion)
	assert.Contains(t, ansi.Strip(m.envBar()), "v1.30.1")
}

func TestContextString(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.category = domain.Tenant
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "foo"}
	m.selectedKey = "foo"
	// In DetailsView the row is the panel's title, not repeated in the path.
	m.viewMode = common.DetailsView
	out := m.contextString()
	assert.Equal(t, "Tenant", out)
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
	m.textInput.SetValue("input")
	assert.NotContains(t, m.statusView(), "input", "the input shows only while typing")
	m.inputMode = common.EditInput
	assert.Contains(t, m.statusView(), "input")
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
	m.detailJSON = true // the renderer draws the JSON layout
	cmd := m.updateContentAsync()
	require.NotNil(t, cmd)
	msg, ok := cmd().(detailContentRenderedMsg)
	require.True(t, ok)
	m.handleDetailContentRenderedMsg(msg)
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
	out := ansi.Strip(m.render())
	assert.Contains(t, out, "Loading Tenant")
	assert.Contains(t, out, "|_   _|", "the logo heads the loading screen")
}

func TestEnvBar_LoadingNugget(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.pendingTasks = 1
	out := m.envBar()
	// The stopwatch renders elapsed time; pendingTasks>0 means the
	// inline loading cell should appear with a duration token like "0s".
	assert.Contains(t, ansi.Strip(out), "0s")
	// Sanity-check that pendingTasks=0 omits the nugget.
	m.pendingTasks = 0
	assert.NotContains(t, ansi.Strip(m.envBar()), "0s")
}

func TestView_HelpView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.HelpView
	out := ansi.Strip(m.render())
	assert.Contains(t, out, "General")
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
