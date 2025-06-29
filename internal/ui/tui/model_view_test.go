package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
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
	m.choice = "dev-UNKNOWN"
	m.updateContent(80)
	viewStr := m.View()
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
}

func TestContextString(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.category = domain.Tenant
	m.context = &domain.ToolkitContext{Category: domain.Tenant, Name: "foo"}
	m.choice = "foo"
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
	m.contextStyle = lipgloss.NewStyle()
	m.statsStyle = lipgloss.NewStyle()
	m.statusText = lipgloss.NewStyle()
	m.textInput.SetValue("input")
	out := m.statusView()
	assert.Contains(t, out, "input")
}

func TestUpdateContent_DetailsView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.DetailsView
	m.choice = "foo"
	m.dataset = &models.Dataset{}
	m.viewport.SetContent("")
	m.renderer = &testRenderer{}
	m.updateContent(40)
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
	m.choice = "foo"
	m.dataset = &models.Dataset{}
	m.viewport.SetContent("")
	m.renderer = &errRenderer{}
	m.updateContent(40)
	assert.Error(t, m.err)
}

type errRenderer struct{}

func (errRenderer) RenderJSON(_ any, _ int) (string, error) {
	return "", errors.New("fail")
}

func TestView_PendingTasks(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.pendingTasks = 1
	out := m.View()
	assert.Contains(t, out, "Loading data")
}

func TestView_Error(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.err = errors.New("fail")
	out := m.View()
	assert.Contains(t, out, "fail")
}

func TestView_HelpView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.HelpView
	out := m.View()
	assert.Contains(t, out, "Global Actions")
}

func TestView_ListView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.ListView
	out := m.View()
	assert.NotEmpty(t, out)
}

func TestView_DetailsView(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.viewMode = common.DetailsView
	m.choice = "foo"
	m.viewport.SetContent("details")
	out := m.View()
	assert.Contains(t, out, "details")
}
