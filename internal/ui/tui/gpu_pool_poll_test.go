package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/oci"
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

// A requested scale-up reports its sizes, and that one past the
// Terraform size lasts until the next terraform apply.
func TestHandleGPUPoolScaleResultMsg_ReportsSizes(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotNil(t, m.handleGPUPoolScaleResultMsg(gpuPoolScaleResultMsg{
		key: "pool-a", env: m.environment, declared: 4, scale: oci.PoolScale{Shown: 2, Before: 2, After: 4},
	}))
	assert.Equal(t, "scale-up requested for pool-a: 2 → 4 instances", m.toasts.Active().Msg)

	m.handleGPUPoolScaleResultMsg(gpuPoolScaleResultMsg{
		key: "pool-a", env: m.environment, declared: 4, scale: oci.PoolScale{Shown: 4, Before: 5, After: 6},
	})
	assert.Contains(t, m.toasts.Active().Msg, "5 → 6 instances (was 5, not 4)")
	assert.Contains(t, m.toasts.Active().Msg, "past its Terraform size (4) until the next terraform apply")
}
