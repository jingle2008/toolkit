package tui

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/pkg/models"
)

// gpuPoolPollInterval is how often the GPU pools' OCI state is re-fetched
// while one of them is mid-transition.
const gpuPoolPollInterval = 15 * time.Second

// gpuPoolPollMsg fires a scheduled GPU pool enrichment for env.
type gpuPoolPollMsg struct {
	env models.Environment
}

// isTransitionalPoolState reports whether an OCI instance-pool lifecycle
// state is one the pool moves out of on its own (e.g. SCALING → RUNNING).
func isTransitionalPoolState(status string) bool {
	switch status {
	case "PROVISIONING", "SCALING", "STARTING", "STOPPING", "TERMINATING":
		return true
	}
	return false
}

// scheduleGPUPoolPoll schedules the next enrichment when a pool is
// mid-transition and no poll is already pending; otherwise it returns nil,
// ending the poll chain once every pool has settled.
func (m *Model) scheduleGPUPoolPoll() tea.Cmd {
	if m.gpuPoolPollPending || m.dataset == nil ||
		!slices.ContainsFunc(m.dataset.GPUPools, func(p models.GPUPool) bool { return isTransitionalPoolState(p.Status) }) {
		return nil
	}
	m.gpuPoolPollPending = true
	env := m.environment
	return tea.Tick(gpuPoolPollInterval, func(time.Time) tea.Msg { return gpuPoolPollMsg{env: env} })
}

// handleGPUPoolPoll runs the scheduled enrichment. A tick from a previous
// environment is dropped, but it re-checks the current one: its pending
// flag would otherwise have kept the new environment from scheduling.
func (m *Model) handleGPUPoolPoll(msg gpuPoolPollMsg) tea.Cmd {
	m.gpuPoolPollPending = false
	if !m.inCurrentEnv(msg.env) {
		return m.scheduleGPUPoolPoll()
	}
	return m.updateGPUPoolState()
}
