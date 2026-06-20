package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// collectCmdsMsgTypes runs each cmd in a slice and returns all produced
// message type strings (flattening BatchMsg one level deep).
func collectCmdsMsgTypes(t *testing.T, cmds []tea.Cmd) []string {
	t.Helper()
	var types []string
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch m := msg.(type) {
		case tea.BatchMsg:
			for _, c := range m {
				if c == nil {
					continue
				}
				types = append(types, msgTypeName(c()))
			}
		default:
			types = append(types, msgTypeName(msg))
		}
	}
	return types
}

func msgTypeName(msg tea.Msg) string { return fmt.Sprintf("%T", msg) }
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// hasWatchLifecycle returns true if the slice contains a watchStartedMsg or
// watchUnavailableMsg type string.
func hasWatchLifecycle(types []string) bool {
	return contains(types, "tui.watchStartedMsg") || contains(types, "tui.watchUnavailableMsg")
}

// TestUpdateCategoryCore_GPUNode_CacheMiss asserts that entering GPUNode with
// no cached data starts both a load command and a watch command.
func TestUpdateCategoryCore_GPUNode_CacheMiss(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Start from a different category so updateCategoryCore treats it as a
	// true navigation (not same-category refresh).
	m.category = domain.Environment
	// Ensure there is no cached GPUNode data.
	m.dataset = &models.Dataset{}

	cmds := m.updateCategoryCore(domain.GPUNode)
	require.NotEmpty(t, cmds)

	types := collectCmdsMsgTypes(t, cmds)
	assert.True(t,
		contains(types, "tui.gpuNodesLoadedMsg"),
		"expected gpuNodesLoadedMsg, got %v", types)
	assert.True(t,
		hasWatchLifecycle(types),
		"expected a watch lifecycle message, got %v", types)
}

// TestUpdateCategoryCore_GPUNode_Cached asserts that re-entering GPUNode
// when data is already loaded does NOT increment pendingTasks (no task leak),
// but still starts a watch.
func TestUpdateCategoryCore_GPUNode_Cached(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Start from a different category.
	m.category = domain.Environment
	// Pre-populate the cache so the handler returns nil (no load needed).
	m.dataset = &models.Dataset{
		GPUNodeMap: map[string][]models.GPUNode{
			"pool1": {},
		},
	}

	cmds := m.updateCategoryCore(domain.GPUNode)
	require.NotEmpty(t, cmds)

	// Run all cmds to produce messages. beginTask is only called during cmd
	// execution for new load tasks; on the cached path it was never called,
	// so pendingTasks stays 0.
	types := collectCmdsMsgTypes(t, cmds)

	// The handler returned nil → beginTask must NOT have been called.
	assert.Equal(t, 0, m.pendingTasks,
		"cached re-entry must not leak a pending task")

	// Watch should still be started even when the load was skipped.
	assert.True(t,
		hasWatchLifecycle(types),
		"expected a watch lifecycle message even on cached path, got %v", types)
}
