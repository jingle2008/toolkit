package tui

import (
	"testing"

	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestDebounceFilter(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.textInput.SetValue("FOO")
	cmd := DebounceFilter(m)
	assert.NotNil(t, cmd)
	// Simulate the Tick firing
	msg := cmd()
	filterMsg, ok := msg.(FilterMsg)
	assert.True(t, ok)
	assert.Equal(t, "foo", filterMsg.Text)
}
