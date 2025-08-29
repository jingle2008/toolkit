package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestLoadRequest_Run(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	lr := loadRequest{
		category:    domain.BaseModel,
		loader:      m.loader,
		ctx:         m.ctx,
		repoPath:    m.repoPath,
		kubeConfig:  m.kubeConfig,
		environment: m.environment,
	}
	cmd := lr.Run()
	assert.NotNil(t, cmd)
}

func TestModelUpdate_QuitKey(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	assert.NotNil(t, cmd)
}

func TestModelUpdate_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, cmd := m.Update(msg)
	assert.Nil(t, cmd)
	assert.True(t, m.reLayout)
}

func TestModelReduce_ViewModes(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	// HelpView
	m.viewMode = common.HelpView
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	assert.Nil(t, cmd)
	// ListView
	m.viewMode = common.ListView
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	assert.Nil(t, cmd)
	// DetailsView
	m.viewMode = common.DetailsView
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	assert.Nil(t, cmd)
}

func TestEnterEditModeAndExitEditMode(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	m.enterEditMode(common.FilterTarget)
	assert.Equal(t, common.EditInput, m.inputMode)
	assert.Equal(t, common.FilterTarget, m.editTarget)
	m.exitEditMode(true)
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Equal(t, common.NoneTarget, m.editTarget)
}
