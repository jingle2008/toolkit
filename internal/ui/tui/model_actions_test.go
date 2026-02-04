package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestToggleFaultyList(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GpuNode
	m.dataset.GpuNodeMap = map[string][]models.GpuNode{
		"pool": {{Name: "node1", InstanceType: "gpu.1"}},
	}
	m.updateColumns()
	m.updateRows(true)

	if m.showFaulty {
		t.Fatal("expected showFaulty to start false")
	}

	m.toggleFaultyList()
	if !m.showFaulty {
		t.Fatal("expected showFaulty to toggle true")
	}
}

func TestHandleItemActions_Refresh(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GpuPool
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.dataset.GpuPools = []models.GpuPool{{Name: "pool1"}}
	m.updateColumns()
	m.updateRows(true)

	cmd := m.handleItemActions(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh to return a cmd")
	}
}

func TestScaleUpGpuPool_InvalidItem(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	if cmd := m.scaleUpGpuPool(nil); cmd != nil {
		t.Fatal("expected nil cmd for nil pool")
	}
	if cmd := m.scaleUpGpuPool("not-a-pool"); cmd != nil {
		t.Fatal("expected nil cmd for invalid pool type")
	}
}

func TestCordonAndDrainNode_InvalidItem(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	if cmd := m.cordonNode(nil); cmd != nil {
		t.Fatal("expected nil cmd for nil cordon item")
	}
	if cmd := m.cordonNode("not-a-node"); cmd != nil {
		t.Fatal("expected nil cmd for invalid cordon item type")
	}
	if cmd := m.drainNode(nil); cmd != nil {
		t.Fatal("expected nil cmd for nil drain item")
	}
	if cmd := m.drainNode("not-a-node"); cmd != nil {
		t.Fatal("expected nil cmd for invalid drain item type")
	}
}

func TestDeleteItemActions(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	m.category = domain.DedicatedAICluster
	m.dataset.DedicatedAIClusterMap = map[string][]models.DedicatedAICluster{
		"tenant1": {{Name: "dac1", Status: "RUNNING"}},
	}
	m.updateColumns()
	m.updateRows(true)
	key := models.ScopedItemKey{Name: "dac1", Scope: "tenant1"}
	cmd := m.deleteItem(key)
	if cmd == nil {
		t.Fatal("expected deleteItem to return cmd for dedicated AI cluster")
	}
	dac := findItem(m.dataset, m.category, key).(*models.DedicatedAICluster)
	if dac.Status != "Deleting" {
		t.Fatalf("expected dedicated AI cluster status Deleting, got %q", dac.Status)
	}

	m.category = domain.GpuNode
	m.dataset.GpuNodeMap = map[string][]models.GpuNode{
		"pool": {{Name: "node1", InstanceType: "gpu.1"}},
	}
	m.updateColumns()
	m.updateRows(true)
	key = models.ScopedItemKey{Name: "node1", Scope: "pool"}
	cmd = m.deleteItem(key)
	if cmd == nil {
		t.Fatal("expected deleteItem to return cmd for gpu node")
	}
	node := findItem(m.dataset, m.category, key).(*models.GpuNode)
	if node.GetStatus() != "Deleting" {
		t.Fatalf("expected gpu node status Deleting, got %q", node.GetStatus())
	}
}

func TestRebootNode_InvalidItem(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	if cmd := m.rebootNode(nil); cmd != nil {
		t.Fatal("expected nil cmd for nil reboot item")
	}
	if cmd := m.rebootNode("not-a-node"); cmd != nil {
		t.Fatal("expected nil cmd for invalid reboot item type")
	}
}

func TestHandleUpdateDoneMsg_ErrorUpdatesStatus(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GpuPool
	m.dataset.GpuPools = []models.GpuPool{{Name: "pool1", Status: "RUNNING"}}
	m.updateColumns()
	m.updateRows(true)

	m.handleUpdateDoneMsg(updateDoneMsg{err: errors.New("boom"), category: domain.GpuPool})
	if got := m.dataset.GpuPools[0].Status; got != "UNKNOWN" {
		t.Fatalf("expected gpu pool status UNKNOWN, got %q", got)
	}
}

func TestRegionalOverrideHandlers(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.LimitRegionalOverrides = []models.LimitRegionalOverride{{Name: "l1"}}
	m.dataset.ConsolePropertyRegionalOverrides = []models.ConsolePropertyRegionalOverride{{Name: "c1"}}
	m.dataset.PropertyRegionalOverrides = []models.PropertyRegionalOverride{{Name: "p1"}}

	if cmd := m.handleLimitRegionalOverrideCategory(1); cmd != nil {
		t.Fatal("expected nil cmd when limit regional overrides already loaded")
	}
	if cmd := m.handleConsolePropertyRegionalOverrideCategory(1); cmd != nil {
		t.Fatal("expected nil cmd when console property regional overrides already loaded")
	}
	if cmd := m.handlePropertyRegionalOverrideCategory(1); cmd != nil {
		t.Fatal("expected nil cmd when property regional overrides already loaded")
	}

	m.dataset.LimitRegionalOverrides = nil
	m.dataset.ConsolePropertyRegionalOverrides = nil
	m.dataset.PropertyRegionalOverrides = nil

	cmd := m.handleLimitRegionalOverrideCategory(2)
	if cmd == nil {
		t.Fatal("expected cmd when limit regional overrides missing")
	}
	require.IsType(t, limitRegionalOverridesLoadedMsg{}, cmd())

	cmd = m.handleConsolePropertyRegionalOverrideCategory(2)
	if cmd == nil {
		t.Fatal("expected cmd when console property regional overrides missing")
	}
	require.IsType(t, consolePropertyRegionalOverridesLoadedMsg{}, cmd())

	cmd = m.handlePropertyRegionalOverrideCategory(2)
	if cmd == nil {
		t.Fatal("expected cmd when property regional overrides missing")
	}
	require.IsType(t, propertyRegionalOverridesLoadedMsg{}, cmd())
}

func TestGetViewportBinding(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	bindings := m.getViewportBinding()
	if len(bindings) != 6 {
		t.Fatalf("expected 6 viewport bindings, got %d", len(bindings))
	}
	if bindings[0].Help().Key != m.viewport.KeyMap.Up.Help().Key {
		t.Fatalf("expected first binding to match viewport up key")
	}
}

func TestCopyItemJSON_ErrorPath(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("copyItemJSON panicked: %v", r)
		}
	}()

	m.copyItemJSON(make(chan int))
}
