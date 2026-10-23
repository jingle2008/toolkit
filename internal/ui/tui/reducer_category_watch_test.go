package tui

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

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

// blockingDACLoader holds LoadDedicatedAIClusters until release is closed,
// standing in for a slow cluster.
type blockingDACLoader struct {
	fakeLoader
	release chan struct{}
}

func (b blockingDACLoader) LoadDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.DedicatedAICluster, error) {
	<-b.release
	return b.fakeLoader.LoadDedicatedAIClusters(ctx, kubeCfg, env)
}

// runLikeProgram executes cmd the way tea.Program does: a sequence's cmds
// one at a time, each waiting for the last, and a batch's concurrently,
// sending every message to out as it's produced. sequenceMsg is unexported,
// so it's recognized by shape, a slice of cmds.
func runLikeProgram(cmd tea.Cmd, out chan<- tea.Msg) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var wg sync.WaitGroup
		for _, c := range msg {
			wg.Go(func() { runLikeProgram(c, out) })
		}
		wg.Wait()
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
			for i := range v.Len() {
				runLikeProgram(v.Index(i).Interface().(tea.Cmd), out)
			}
			return
		}
		out <- msg
	}
}

// TestUpdateCategory_CachedRowsDontWaitForReload: callers run a category's
// cmds in a tea.Sequence, and the cached rows must show while the reload is
// still out, not after it lands (the list was blank on return from the
// Alias list until the DACs reloaded).
func TestUpdateCategory_CachedRowsDontWaitForReload(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.Alias
	m.dataset.DedicatedAIClusterMap = map[string][]models.DedicatedAICluster{
		"tenant1": {{Name: "dac1"}},
	}
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock) // a failed wait mustn't leave the load blocked
	m.loader = blockingDACLoader{release: release}

	out := make(chan tea.Msg, 64)
	done := make(chan struct{})
	go func() {
		runLikeProgram(tea.Sequence(m.updateCategory(domain.DedicatedAICluster)...), out)
		close(done)
	}()

	waitFor := func(want string) {
		t.Helper()
		timeout := time.After(2 * time.Second)
		for {
			select {
			case msg := <-out:
				if msgTypeName(msg) == want {
					return
				}
			case <-timeout:
				t.Fatalf("no %s within 2s", want)
			}
		}
	}
	waitFor("tui.dataMsg") // the reload is still blocked
	unblock()
	waitFor("tui.dedicatedAIClustersLoadedMsg")
	<-done
}
