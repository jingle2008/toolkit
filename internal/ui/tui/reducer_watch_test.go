package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestHandleWatchStarted_SetsWatchingAndArms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t) // existing TUI test constructor
	m.gen = 4
	m.category = domain.GPUNode
	trig := make(chan struct{}, 1)

	cmd := m.handleWatchStarted(watchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 4})
	assert.True(t, m.watching)
	require.NotNil(t, cmd, "must re-arm the trigger listener")
}

func TestHandleWatchStarted_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 5
	trig := make(chan struct{}, 1)

	cmd := m.handleWatchStarted(watchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 2})
	assert.False(t, m.watching, "stale watchStartedMsg must not enable watching")
	assert.Nil(t, cmd)
}

func TestHandleWatchTriggered_ReloadsAndRearms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 3
	m.category = domain.GPUNode
	m.newLoadContext()
	// Store a trigger channel so waitForTrigger can re-arm
	trig := make(chan struct{}, 1)
	m.watchTrigger = trig

	cmd := m.handleWatchTriggered(watchTriggeredMsg{Cat: domain.GPUNode, Gen: 3})
	require.NotNil(t, cmd, "trigger must produce reload + re-arm cmds")
}

func TestHandleWatchTriggered_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 8
	cmd := m.handleWatchTriggered(watchTriggeredMsg{Cat: domain.GPUNode, Gen: 1})
	assert.Nil(t, cmd)
}

func TestHandleWatchClosed_ClearsWatchingAndReloads(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.category = domain.GPUNode
	m.watching = true
	m.newLoadContext()

	cmd := m.handleWatchClosed(watchClosedMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.watching, "closed watch clears the live indicator")
	require.NotNil(t, cmd, "closed watch issues one final reload")
}

func TestHandleWatchUnavailable_ClearsWatchingWhenActive(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.watching = true
	m.handleWatchUnavailable(watchUnavailableMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.watching, "unavailable must clear the live indicator")
}
