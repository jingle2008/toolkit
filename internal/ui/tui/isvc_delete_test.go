package tui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func ctrlX() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlX} }

func stubOrphanPlan(t *testing.T, outcome k8s.OrphanDeleteOutcome, err error) {
	t.Helper()
	orig := planOrphanISVCDeleteFn
	t.Cleanup(func() { planOrphanISVCDeleteFn = orig })
	planOrphanISVCDeleteFn = func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
		return outcome, err
	}
}

// ctrl+x checks the cluster first, then opens the irreversible modal
// naming the planned action.
//
//nolint:paralleltest // swaps the package-level plan seam
func TestCtrlX_InInferenceService_PlansThenConfirms(t *testing.T) {
	for _, tc := range []struct {
		outcome k8s.OrphanDeleteOutcome
		action  string
	}{
		{k8s.OrphanDeleted, "Delete"},
		{k8s.OrphanFinalized, "Force-delete"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			m := newInferenceServiceModel(t)
			stubOrphanPlan(t, tc.outcome, nil)

			cmd := m.handleItemActions(ctrlX())
			require.NotNil(t, cmd, "ctrl+x must plan first")
			assert.Equal(t, common.ListView, m.viewMode, "no modal before the plan lands")
			m.handleISVCDeletePlannedMsg(cmd().(isvcDeletePlannedMsg))
			require.Equal(t, common.ConfirmView, m.viewMode)
			assert.Equal(t, mutation.Irreversible, m.confirm.act.Tier)
			assert.Equal(t, tc.action, m.confirm.action)
			assert.Equal(t, "ns1/svc1", m.confirm.target)
		})
	}
}

//nolint:paralleltest // swaps the package-level plan seam
func TestCtrlX_InInferenceService_RefusalToasts(t *testing.T) {
	m := newInferenceServiceModel(t)
	stubOrphanPlan(t, "", fmt.Errorf("%w: its DAC dac-1 exists (ome.io); delete the DAC instead", k8s.ErrNotOrphan))

	cmd := m.handleItemActions(ctrlX())
	require.NotNil(t, cmd)
	m.handleISVCDeletePlannedMsg(cmd().(isvcDeletePlannedMsg))
	assert.Equal(t, common.ListView, m.viewMode, "a refusal opens no modal")
	require.NotNil(t, m.toasts.active)
	assert.Contains(t, m.toasts.active.msg, "delete the DAC instead")
}

// A plan landing after the user left the list doesn't pop a modal.
//
//nolint:paralleltest // swaps the package-level plan seam
func TestCtrlX_PlanAfterNavigationIsDropped(t *testing.T) {
	m := newInferenceServiceModel(t)
	stubOrphanPlan(t, k8s.OrphanDeleted, nil)
	cmd := m.handleItemActions(ctrlX())
	m.category = domain.DedicatedAICluster
	m.handleISVCDeletePlannedMsg(cmd().(isvcDeletePlannedMsg))
	assert.Equal(t, common.ListView, m.viewMode)
}

//nolint:paralleltest // swaps the package-level seams
func TestDeleteOrphanInferenceService_ConfirmDeletesAndToasts(t *testing.T) {
	m := newInferenceServiceModel(t)
	stubOrphanPlan(t, k8s.OrphanFinalized, nil)
	var gotNS, gotName string
	orig := deleteOrphanISVCFn
	t.Cleanup(func() { deleteOrphanISVCFn = orig })
	deleteOrphanISVCFn = func(_ context.Context, _, _, namespace, name string) (k8s.OrphanDeleteOutcome, error) {
		gotNS, gotName = namespace, name
		return k8s.OrphanFinalized, nil
	}

	m.handleISVCDeletePlannedMsg(m.handleItemActions(ctrlX())().(isvcDeletePlannedMsg))
	_, cmd := m.updateConfirmView(keyMsg("y"))
	assert.Nil(t, cmd, "irreversible: lowercase y cancels")

	m.handleISVCDeletePlannedMsg(m.handleItemActions(ctrlX())().(isvcDeletePlannedMsg))
	_, cmd = m.updateConfirmView(keyMsg("Y"))
	require.NotNil(t, cmd, "confirming must dispatch the delete")
	msg, ok := cmd().(isvcDeleteResultMsg)
	require.True(t, ok)
	assert.Equal(t, "ns1", gotNS)
	assert.Equal(t, "svc1", gotName)

	m.handleISVCDeleteResultMsg(msg)
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "force-deleted ns1/svc1: finalizer removed", m.toasts.active.msg)

	m.handleISVCDeleteResultMsg(isvcDeleteResultMsg{key: msg.key, err: errors.New("boom")})
	assert.Contains(t, m.toasts.active.msg, "delete ns1/svc1 failed: boom")
}
