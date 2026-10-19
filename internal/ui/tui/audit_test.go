package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// auditRecord is the part of a "mutation" audit record a test checks.
type auditRecord struct{ action, kind, target, phase string }

func auditRecords(sink *logging.RingSink) []auditRecord {
	var out []auditRecord
	for _, e := range sink.Snapshot() {
		if e.Message != "mutation" {
			continue
		}
		kv := map[any]any{}
		for i := 0; i+1 < len(e.Fields); i += 2 {
			kv[e.Fields[i]] = e.Fields[i+1]
		}
		if kv["surface"] != surfaceTUI {
			continue
		}
		out = append(out, auditRecord{
			action: kv["action"].(string), kind: kv["kind"].(string), //nolint:forcetypeassert // Audit writes strings
			target: kv["target"].(string), phase: kv["phase"].(string), //nolint:forcetypeassert // Audit writes strings
		})
	}
	return out
}

func newGPUNodeModel(t *testing.T, node models.GPUNode) (*Model, models.ItemKey, *logging.RingSink) {
	t.Helper()
	m := newTestModel(t)
	sink := logging.NewRingSink(16)
	m.logger = sink
	m.category = domain.GPUNode
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{"pool1": {node}}
	return m, models.ScopedItemKey{Scope: "pool1", Name: node.Name}, sink
}

// The cordon key sets the state the modal names — uncordon for a cordoned
// node — and the TUI audits it as the CLI and MCP do.
//
//nolint:paralleltest // swaps the package-level setCordonFn seam
func TestCordon_ConfirmedIsAudited(t *testing.T) {
	orig := setCordonFn
	t.Cleanup(func() { setCordonFn = orig })
	var gotWant []bool
	setCordonFn = func(_ context.Context, _, _, _ string, want bool) (bool, error) {
		gotWant = append(gotWant, want)
		return true, nil
	}

	for _, tc := range []struct {
		cordoned bool
		verb     string
	}{{false, "cordon"}, {true, "uncordon"}} {
		node := models.GPUNode{Name: "gpu-1", IsSchedulingDisabled: tc.cordoned}
		m, key, sink := newGPUNodeModel(t, node)
		m.requestConfirm(m.confirmCordon(key, &node))
		_, cmd := m.updateConfirmView(keyPress("y"))
		require.NotNil(t, cmd)
		msg, ok := actionResult(t, cmd).(cordonNodeResultMsg)
		require.True(t, ok)
		assert.Equal(t, !tc.cordoned, msg.state, "the state set is the one confirmed")
		assert.Equal(t, []auditRecord{
			{tc.verb, "node", "gpu-1", "begin"},
			{tc.verb, "node", "gpu-1", "done"},
		}, auditRecords(sink))
	}
	assert.Equal(t, []bool{true, false}, gotWant)
}

// A cancelled modal acts on nothing and is audited as refused, as
// declining the CLI's prompt is.
//
//nolint:paralleltest // swaps the package-level setCordonFn seam
func TestCordon_CancelledIsAuditedRefused(t *testing.T) {
	orig := setCordonFn
	t.Cleanup(func() { setCordonFn = orig })
	setCordonFn = func(context.Context, string, string, string, bool) (bool, error) {
		t.Fatal("cancelled cordon ran")
		return false, nil
	}
	node := models.GPUNode{Name: "gpu-1"}
	m, key, sink := newGPUNodeModel(t, node)
	m.requestConfirm(m.confirmCordon(key, &node))
	_, cmd := m.updateConfirmView(keyPress("n"))
	assert.Nil(t, cmd)
	assert.Equal(t, []auditRecord{{"cordon", "node", "gpu-1", "refused"}}, auditRecords(sink))
}

// An orphan delete is audited as the plan named it, under its
// <namespace>/<name>, including a failure.
//
//nolint:paralleltest // swaps the package-level plan and delete seams
func TestISVCDelete_ConfirmedIsAuditedAsPlanned(t *testing.T) {
	stubOrphanPlan(t, k8s.OrphanFinalized, nil)
	orig := deleteOrphanISVCFn
	t.Cleanup(func() { deleteOrphanISVCFn = orig })
	deleteOrphanISVCFn = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		return "", assert.AnError
	}

	m := newInferenceServiceModel(t)
	sink := logging.NewRingSink(16)
	m.logger = sink
	m.handleISVCDeletePlannedMsg(m.handleItemActions(ctrlX())().(isvcDeletePlannedMsg))
	_, cmd := m.updateConfirmView(keyPress("Y"))
	require.NotNil(t, cmd)
	msg, ok := actionResult(t, cmd).(isvcDeleteResultMsg)
	require.True(t, ok)
	require.ErrorIs(t, msg.err, assert.AnError)
	assert.Equal(t, []auditRecord{
		{"force-delete", "inference_service", "ns1/svc1", "begin"},
		{"force-delete", "inference_service", "ns1/svc1", "failed"},
	}, auditRecords(sink))
}

// A refused InferenceService delete names <namespace>/<name>, as its
// done record would.
//
//nolint:paralleltest // swaps the package-level plan seam
func TestISVCDelete_CancelledIsAuditedRefused(t *testing.T) {
	stubOrphanPlan(t, k8s.OrphanDeleted, nil)
	m := newInferenceServiceModel(t)
	sink := logging.NewRingSink(16)
	m.logger = sink
	m.handleISVCDeletePlannedMsg(m.handleItemActions(ctrlX())().(isvcDeletePlannedMsg))
	_, cmd := m.updateConfirmView(keyPress("y")) // lowercase y cancels an irreversible action
	assert.Nil(t, cmd)
	assert.Equal(t, []auditRecord{{"delete", "inference_service", "ns1/svc1", "refused"}}, auditRecords(sink))
}
