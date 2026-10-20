package tui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
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

// hasWatchLifecycle returns true if the slice contains a k8sWatchStartedMsg or
// k8sWatchUnavailableMsg type string.
func hasWatchLifecycle(types []string) bool {
	return contains(types, "tui.k8sWatchStartedMsg") || contains(types, "tui.k8sWatchUnavailableMsg")
}

// TestUpdateCategoryCore_NavigationClearsFilter asserts that navigating to a
// different category resets the active filter and the filter input.
func TestUpdateCategoryCore_NavigationClearsFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.Environment
	m.filter = "stale"
	m.textInput.SetValue("stale")

	_ = m.updateCategoryCore(domain.GPUNode)

	if m.filter != "" {
		t.Fatalf("navigation did not clear filter: %q", m.filter)
	}
	if m.textInput.Value() != "" {
		t.Fatalf("navigation did not reset input: %q", m.textInput.Value())
	}
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
// with data already loaded shows the cached rows at once but also reloads:
// the cache went stale when the category's watch stopped, and the new
// watch only reports changes from now on.
func TestUpdateCategoryCore_GPUNode_Cached(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Start from a different category.
	m.category = domain.Environment
	m.dataset = &models.Dataset{
		GPUNodeMap: map[string][]models.GPUNode{
			"pool1": {},
		},
	}

	cmds := m.updateCategoryCore(domain.GPUNode)
	require.NotEmpty(t, cmds)

	types := collectCmdsMsgTypes(t, cmds)
	assert.True(t, contains(types, "tui.dataMsg"), "cached rows are shown at once, got %v", types)
	assert.True(t, contains(types, "tui.gpuNodesLoadedMsg"), "the stale cache is reloaded, got %v", types)
	assert.Equal(t, 1, m.pendingTasks, "one task for the reload, ended by its result")
	assert.True(t,
		hasWatchLifecycle(types),
		"expected a watch lifecycle message, got %v", types)
}
