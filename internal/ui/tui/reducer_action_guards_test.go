package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// The per-row action reducers all guard on the selected item before
// dispatching anything that touches k8s or OCI. These tests pin the guards
// — a nil command means "nothing was dispatched", which is what keeps a
// stale or mistyped selection from firing a mutation at the cluster.
//
// The success paths reach live infrastructure and are covered by the
// actions/k8s packages plus the *ResultMsg reducer tests, so they're
// deliberately not exercised here.

func TestActionGuards_NilItem(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ItemKey("node-a")

	assert.Nil(t, m.cordonNode(nil, key, mutation.CordonNode), "cordon must not dispatch without an item")
	assert.Nil(t, m.drainNode(nil, key), "drain must not dispatch without an item")
	assert.Nil(t, m.restartNode(nil, key), "restart must not dispatch without an item")
	assert.Nil(t, m.scaleUpGPUPool(nil, key), "scale must not dispatch without an item")
}

func TestActionGuards_WrongItemType(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ItemKey("node-a")

	// A Tenant is a valid item for other categories but not for node or
	// pool actions; the type assertion must reject it rather than panic.
	wrong := &models.Tenant{Name: "tenant1"}

	assert.Nil(t, m.cordonNode(wrong, key, mutation.CordonNode))
	assert.Nil(t, m.drainNode(wrong, key))
	assert.Nil(t, m.restartNode(wrong, key))
	assert.Nil(t, m.scaleUpGPUPool(wrong, key))
}

func TestRestartNode_AlreadyRestartingIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	node := &models.GPUNode{Name: "node-a"}
	node.SetStatus("Restarting")

	// Re-pressing the key while a restart is in flight must not queue a
	// second SoftResetInstance for the same node.
	assert.Nil(t, m.restartNode(node, models.ItemKey("node-a")))
}

func TestDeleteGPUNode_ItemNotFound(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// The key doesn't resolve in the dataset, so findItem returns nil and
	// the reducer must bail instead of terminating an instance it guessed at.
	assert.Nil(t, m.deleteGPUNode(models.ItemKey("no-such-node")))
}
