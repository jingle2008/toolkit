package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleWatchStarted_SetsWatchingAndArms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t) // existing TUI test constructor
	m.gen = 4
	m.category = domain.GPUNode
	trig := make(chan struct{}, 1)

	cmd := m.handleK8sWatchStarted(k8sWatchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 4})
	assert.True(t, m.k8sWatching)
	require.NotNil(t, cmd, "must re-arm the trigger listener")
}

func TestHandleWatchStarted_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 5
	trig := make(chan struct{}, 1)

	cmd := m.handleK8sWatchStarted(k8sWatchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 2})
	assert.False(t, m.k8sWatching, "stale k8sWatchStartedMsg must not enable watching")
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
	m.k8sWatchTrigger = trig

	cmd := m.handleK8sWatchTriggered(k8sWatchTriggeredMsg{Cat: domain.GPUNode, Gen: 3})
	require.NotNil(t, cmd, "trigger must produce reload + re-arm cmds")
}

func TestHandleWatchTriggered_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 8
	cmd := m.handleK8sWatchTriggered(k8sWatchTriggeredMsg{Cat: domain.GPUNode, Gen: 1})
	assert.Nil(t, cmd)
}

func TestHandleWatchClosed_ClearsWatchingAndReloads(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.category = domain.GPUNode
	m.k8sWatching = true
	m.newLoadContext()

	cmd := m.handleK8sWatchClosed(k8sWatchClosedMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.k8sWatching, "closed watch clears the live indicator")
	require.NotNil(t, cmd, "closed watch issues one final reload")
}

func TestHandleWatchUnavailable_ClearsWatchingWhenActive(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.k8sWatching = true
	m.handleK8sWatchUnavailable(k8sWatchUnavailableMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.k8sWatching, "unavailable must clear the live indicator")
}

func TestHandleWatchClosed_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 6
	m.k8sWatching = true

	cmd := m.handleK8sWatchClosed(k8sWatchClosedMsg{Cat: domain.GPUNode, Gen: 3})
	assert.Nil(t, cmd)
	assert.True(t, m.k8sWatching, "stale k8sWatchClosedMsg must not clear the live indicator")
}

func TestHandleWatchUnavailable_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 7
	m.k8sWatching = true

	m.handleK8sWatchUnavailable(k8sWatchUnavailableMsg{Cat: domain.GPUNode, Gen: 4})
	assert.True(t, m.k8sWatching, "stale k8sWatchUnavailableMsg must not clear the live indicator")
}

// A live-watch reload of the on-screen category preserves the active filter and
// the selected row — it does not behave like a navigation.
func TestLiveReload_PreservesFilterAndSelection(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 1
	m.category = domain.BaseModel
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}}
	m.refreshDisplay()
	m.table.SetCursor(1) // select bm2
	// "bm" matches all three items so the table stays populated while we
	// verify the filter string is not cleared by the reload.
	m.filter = "bm"

	// Simulate the data landing from a watch-triggered reload (same gen, same
	// category) via the typed loaded-handler the reload command resolves to.
	m.handleBaseModelsLoaded([]models.BaseModel{
		{Name: "bm1"}, {Name: "bm2"}, {Name: "bm3"},
	}, 1)

	if m.filter != "bm" {
		t.Fatalf("live reload cleared the filter: %q", m.filter)
	}
	if got := m.selectedRawRow(); len(got) == 0 || got[0] != "bm2" {
		t.Fatalf("live reload lost the selection: %v", got)
	}
}
