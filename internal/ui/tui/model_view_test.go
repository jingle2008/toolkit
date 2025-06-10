package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
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
	m.chosen = true
	m.choice = "dev-UNKNOWN"
	m.updateContent(80)
	viewStr := m.View()
	assert.IsType(t, "", viewStr)
}
