package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
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
	applyMsg, ok := msg.(FilterApplyMsg)
	assert.True(t, ok)
	assert.Equal(t, "foo", applyMsg.Value)
	assert.Greater(t, applyMsg.Nonce, 0)
}
