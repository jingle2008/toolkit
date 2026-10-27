package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTypedLoadBaseModelsCmd_Run(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	gen := m.bumpGen()
	cmd := m.loadCategoryCmd(domain.BaseModel, gen)
	msg := cmd()
	assert.NotNil(t, msg)
}

func TestModelUpdate_QuitKey(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	msg := keyPress("ctrl+c")
	_, cmd := m.Update(msg)
	assert.NotNil(t, cmd)
}

func TestModelUpdate_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, cmd := m.Update(msg)
	assert.Nil(t, cmd)
	assert.Equal(t, 80, m.viewWidth)
	assert.Equal(t, 24, m.viewHeight)
}

func TestModelReduce_ViewModes(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	// HelpView
	m.viewMode = common.HelpView
	_, cmd := m.Update(keyPress("h"))
	assert.Nil(t, cmd)
	// ListView
	m.viewMode = common.ListView
	_, cmd = m.Update(keyPress("l"))
	assert.Nil(t, cmd)
	// DetailsView
	m.viewMode = common.DetailsView
	_, cmd = m.Update(keyPress("d"))
	assert.Nil(t, cmd)
}

func TestEnterEditModeAndExitEditMode(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	m.enterFilterMode()
	assert.Equal(t, common.EditInput, m.inputMode)
	assert.Equal(t, common.FilterTarget, m.editTarget)
	m.exitEditMode(true)
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Equal(t, common.NoneTarget, m.editTarget)
}
