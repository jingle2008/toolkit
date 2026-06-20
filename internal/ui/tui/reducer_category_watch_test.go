package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectMsgTypes runs a (possibly batched) cmd and returns the message
// types it produced. tea.Batch returns a BatchMsg of sub-commands.
func collectMsgTypes(t *testing.T, cmd tea.Cmd) []string {
	t.Helper()
	if cmd == nil {
		return nil
	}
	var types []string
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

func TestHandleGPUNodeCategory_BatchesLoadAndWatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.newLoadContext()
	m.dataset = nil // force a load

	cmd := m.handleGPUNodeCategory(true, m.bumpGen())
	require.NotNil(t, cmd)
	types := collectMsgTypes(t, cmd)
	// Expect both a load result and a watch lifecycle message.
	assert.Contains(t, types, "tui.gpuNodesLoadedMsg")
	assert.True(t,
		contains(types, "tui.watchStartedMsg") || contains(types, "tui.watchUnavailableMsg"),
		"expected a watch lifecycle message, got %v", types)
}
