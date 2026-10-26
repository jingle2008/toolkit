package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

func newInferenceServiceModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.category = domain.InferenceService
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.dataset.InferenceServiceMap = map[string][]models.InferenceService{
		"ns1": {{Name: "svc1", Namespace: "ns1", Ready: "False"}},
	}
	m.updateColumns()
	m.updateRows(true)
	return m
}

func shiftR() tea.KeyMsg { return keyPress("R") }

// "R" restarts the selected row; in the InferenceService category that is
// a rolling restart of the endpoint.
func TestShiftR_InInferenceService_OpensRestartConfirm(t *testing.T) {
	t.Parallel()
	m := newInferenceServiceModel(t)

	assert.Nil(t, m.handleItemActions(shiftR()))
	require.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, mutation.Recoverable, m.confirm.Request().Action.Tier)
	assert.Equal(t, "Restart", m.confirm.Request().Verb())
	assert.Equal(t, "InferenceService", m.confirm.Request().Noun())
	assert.Equal(t, "ns1/svc1", m.confirm.Request().Target)
	assert.Equal(t, "Ready: False", m.confirm.Request().Warning)
	assert.Contains(t, m.confirmView(), "Ready: False", "recoverable modal renders the warning line")
}

func TestShiftR_InGPUNode_RestartsNode(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUNode
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{"pool": {{Name: "node1"}}}
	m.updateColumns()
	m.updateRows(true)

	m.handleItemActions(shiftR())
	assert.Equal(t, "Restart", m.confirm.Request().Verb())
	assert.Equal(t, "node", m.confirm.Request().Noun())
}

//nolint:paralleltest // swaps the package-level restartInferenceServiceFn seam
func TestRestartInferenceService_ConfirmPatchesRow(t *testing.T) {
	m := newInferenceServiceModel(t)
	var gotNS, gotName string
	orig := restartInferenceServiceFn
	t.Cleanup(func() { restartInferenceServiceFn = orig })
	restartInferenceServiceFn = func(_ context.Context, _, _, namespace, name string) error {
		gotNS, gotName = namespace, name
		return nil
	}

	m.handleItemActions(shiftR())
	_, cmd := m.updateConfirmView(keyPress("y"))
	require.NotNil(t, cmd, "confirming must dispatch the restart")
	msg, ok := actionResult(t, cmd).(restartServiceResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Equal(t, "ns1", gotNS)
	assert.Equal(t, "svc1", gotName)
}

func TestRestartInferenceService_Guards(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ScopedItemKey{Scope: "ns1", Name: "svc1"}
	assert.Nil(t, m.restartInferenceService(nil, key), "no item, no dispatch")
	assert.Nil(t, m.restartInferenceService(&models.Tenant{Name: "t"}, key), "wrong type, no dispatch")
}

func TestHandleRestartServiceResultMsg_Toasts(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ScopedItemKey{Scope: "ns1", Name: "svc1"}

	require.NotNil(t, m.handleRestartServiceResultMsg(restartServiceResultMsg{key: key, env: m.environment}))
	assert.Equal(t, "restart requested for ns1/svc1", m.toasts.Active().Msg)
	assert.Equal(t, toastInfo, m.toasts.Active().Sev)

	m.handleRestartServiceResultMsg(restartServiceResultMsg{key: key, err: errors.New("forbidden"), env: m.environment})
	assert.Equal(t, "restart ns1/svc1 failed: forbidden", m.toasts.Active().Msg)
	assert.Equal(t, toastError, m.toasts.Active().Sev)
	assert.Equal(t, "R retry", m.toasts.Active().Next)
}

// TestRestartInferenceService_LegacyDispatches pins that a legacy
// (ome.oracle.com/v1alpha1) row restarts like any other: the k8s layer
// picks the API, so the TUI just dispatches namespace/name.
//
//nolint:paralleltest // swaps the package-level restartInferenceServiceFn seam
func TestRestartInferenceService_LegacyDispatches(t *testing.T) {
	m := newTestModel(t)
	var called bool
	orig := restartInferenceServiceFn
	t.Cleanup(func() { restartInferenceServiceFn = orig })
	restartInferenceServiceFn = func(_ context.Context, _, _, namespace, name string) error {
		called = namespace == "ns1" && name == "svc1"
		return nil
	}

	key := models.ScopedItemKey{Scope: "ns1", Name: "svc1"}
	cmd := m.restartInferenceService(&models.InferenceService{Name: "svc1", Namespace: "ns1", Legacy: true}, key)
	require.NotNil(t, cmd)
	msg, ok := actionResult(t, cmd).(restartServiceResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.True(t, called)
}

func TestHandleDrainNodeResultMsg_Toasts(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ScopedItemKey{Scope: "pool1", Name: "node1"}

	require.NotNil(t, m.handleDrainNodeResultMsg(drainNodeResultMsg{key: key, env: m.environment}))
	assert.Equal(t, "drained pool1/node1", m.toasts.Active().Msg)
	assert.Equal(t, toastInfo, m.toasts.Active().Sev)

	m.handleDrainNodeResultMsg(drainNodeResultMsg{key: key, err: errors.New("pdb violation (node uncordoned)"), env: m.environment})
	assert.Equal(t, "drain pool1/node1 failed: pdb violation (node uncordoned)", m.toasts.Active().Msg)
	assert.Equal(t, toastError, m.toasts.Active().Sev)
	assert.Equal(t, "D retry (resumes)", m.toasts.Active().Next, "the drain is recorded on the node")
}
