package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// A pool mid-transition schedules one poll; enrichments landing while it
// is pending don't stack a second chain, and settled pools end it.
func TestHandleUpdateDoneMsg_PollsWhileTransitioning(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a", Size: 4}}
	scaling := []models.GPUPool{{Name: "a", ActualSize: 4, Status: "SCALING"}}

	require.NotNil(t, m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: m.environment, pools: scaling}))
	assert.True(t, m.gpuPoolPollPending)
	assert.Nil(t, m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: m.environment, pools: scaling}), "already pending")

	m.gpuPoolPollPending = false
	running := []models.GPUPool{{Name: "a", ActualSize: 4, Status: "RUNNING"}}
	assert.Nil(t, m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: m.environment, pools: running}))
	assert.False(t, m.gpuPoolPollPending)
}

// A poll tick from a previous environment is dropped but frees the
// pending flag, so the current environment's transitioning pool still
// gets polled.
func TestHandleGPUPoolPoll_PreviousEnvReschedules(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a", Status: "RUNNING"}}
	m.gpuPoolPollPending = true

	assert.Nil(t, m.handleGPUPoolPoll(gpuPoolPollMsg{env: otherEnv}))
	assert.False(t, m.gpuPoolPollPending)

	m.dataset.GPUPools[0].Status = "SCALING"
	m.gpuPoolPollPending = true
	assert.NotNil(t, m.handleGPUPoolPoll(gpuPoolPollMsg{env: otherEnv}))
	assert.True(t, m.gpuPoolPollPending)
}

func TestIsTransitionalPoolState(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"PROVISIONING", "SCALING", "STARTING", "STOPPING", "TERMINATING"} {
		assert.True(t, isTransitionalPoolState(s), s)
	}
	for _, s := range []string{"RUNNING", "STOPPED", "TERMINATED", models.GPUPoolStatusPending, models.GPUPoolStatusUnknown, models.GPUPoolStatusNonexistent} {
		assert.False(t, isTransitionalPoolState(s), s)
	}
}

// A scale-up that sent nothing to OCI says so rather than "requested".
func TestHandleGPUPoolScaleResultMsg_NoopSaysSo(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.handleGPUPoolScaleResultMsg(gpuPoolScaleResultMsg{key: "pool-a", env: m.environment, noop: true})
	require.NotNil(t, m.toasts.active)
	assert.Contains(t, m.toasts.active.msg, "already at its configured size")
}

// A requested scale-up still reports the request.
func TestHandleGPUPoolScaleResultMsg_RequestedSaysSo(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotNil(t, m.handleGPUPoolScaleResultMsg(gpuPoolScaleResultMsg{key: "pool-a", env: m.environment}))
	assert.Contains(t, m.toasts.active.msg, "scale-up requested")
}
