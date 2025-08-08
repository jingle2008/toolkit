package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

type fakeLogger struct{}

func (fakeLogger) Debugw(string, ...any)            {}
func (fakeLogger) Infow(string, ...any)             {}
func (fakeLogger) Errorw(string, ...any)            {}
func (fakeLogger) WithFields(...any) logging.Logger { return fakeLogger{} }
func (fakeLogger) DebugEnabled() bool               { return false }
func (fakeLogger) Sync() error                      { return nil }

func TestNewModel_Valid(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "rl"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Equal(t, "repo", m.repoPath)
	assert.Equal(t, "r", m.environment.Region)
	assert.Equal(t, "t", m.environment.Type)
	assert.Equal(t, "rl", m.environment.Realm)
	assert.NotNil(t, m.table)
	assert.NotNil(t, m.textInput)
	assert.NotNil(t, m.viewport)
	assert.NotNil(t, m.help)
	assert.NotNil(t, m.loadingSpinner)
}

func TestNewModel_MissingRepoPath(t *testing.T) {
	t.Parallel()
	_, err := NewModel(
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "rl"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repoPath")
}

func TestNewModel_MissingEnvironment(t *testing.T) {
	t.Parallel()
	_, err := NewModel(
		WithRepoPath("repo"),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
}

func TestNewModel_MissingLoader(t *testing.T) {
	t.Parallel()
	_, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "rl"}),
		WithLogger(fakeLogger{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loader")
}

func TestNewModel_MissingLogger(t *testing.T) {
	t.Parallel()
	_, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "rl"}),
		WithLoader(fakeLoader{}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logger")
}

func TestSetDefaults_TableStyles(t *testing.T) {
	t.Parallel()
	m := &Model{}
	setDefaults(m)
	assert.NotNil(t, m.table)
	assert.NotNil(t, m.styles)
	assert.NotNil(t, m.textInput)
	assert.NotNil(t, m.viewport)
	assert.NotNil(t, m.help)
	assert.NotNil(t, m.loadingSpinner)
}

func TestInitStyles(t *testing.T) {
	t.Parallel()
	m := &Model{}
	initStyles(m)
	assert.NotNil(t, m.baseStyle)
	assert.NotNil(t, m.statusNugget)
	assert.NotNil(t, m.statusBarStyle)
	assert.NotNil(t, m.contextStyle)
	assert.NotNil(t, m.statsStyle)
	assert.NotNil(t, m.statusText)
	assert.NotNil(t, m.infoKeyStyle)
	assert.NotNil(t, m.infoValueStyle)
	assert.NotNil(t, m.helpBorder)
	assert.NotNil(t, m.helpHeader)
	assert.NotNil(t, m.helpKey)
	assert.NotNil(t, m.helpDesc)
}

func TestApplyOptions(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt1 := func(m *Model) { m.repoPath = "foo" }
	opt2 := func(m *Model) { m.environment = models.Environment{Region: "r", Type: "t", Realm: "rl"} }
	applyOptions(m, []ModelOption{opt1, opt2})
	assert.Equal(t, "foo", m.repoPath)
	assert.Equal(t, "r", m.environment.Region)
}

func TestValidateModel_AllValid(t *testing.T) {
	t.Parallel()
	m := &Model{
		repoPath:    "repo",
		environment: models.Environment{Region: "r", Type: "t", Realm: "rl"},
		loader:      fakeLoader{},
		logger:      fakeLogger{},
	}
	assert.NoError(t, validateModel(m))
}

func TestValidateModel_MissingFields(t *testing.T) {
	t.Parallel()
	m := &Model{}
	err := validateModel(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repoPath")
	m.repoPath = "repo"
	err = validateModel(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
	m.environment = models.Environment{Region: "r", Type: "t", Realm: "rl"}
	err = validateModel(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loader")
	m.loader = fakeLoader{}
	err = validateModel(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logger")
	m.logger = fakeLogger{}
	assert.NoError(t, validateModel(m))
}

func TestSetDefaults_ExistingFields(t *testing.T) {
	t.Parallel()
	m := &Model{
		table:          &table.Model{},
		textInput:      &textinput.Model{},
		viewport:       &viewport.Model{},
		help:           &help.Model{},
		loadingSpinner: &spinner.Model{},
	}
	setDefaults(m)
	assert.NotNil(t, m.table)
	assert.NotNil(t, m.textInput)
	assert.NotNil(t, m.viewport)
	assert.NotNil(t, m.help)
	assert.NotNil(t, m.loadingSpinner)
}

func TestNewModel_OptionsApplied(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Region: "r", Type: "t", Realm: "rl"}),
		WithLoader(fakeLoader{}),
		WithLogger(fakeLogger{}),
	)
	require.NoError(t, err)
	assert.Equal(t, "repo", m.repoPath)
	assert.Equal(t, "r", m.environment.Region)
	assert.Equal(t, "t", m.environment.Type)
	assert.Equal(t, "rl", m.environment.Realm)
}
