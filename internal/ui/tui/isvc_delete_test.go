package tui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func ctrlX() tea.KeyMsg { return keyPress("ctrl+x") }

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
			assert.Equal(t, mutation.Irreversible, m.confirm.Request().Action.Tier)
			assert.Equal(t, tc.action, m.confirm.Request().Verb())
			assert.Equal(t, "ns1/svc1", m.confirm.Request().Target)
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
	require.NotNil(t, m.toasts.Active())
	assert.Contains(t, m.toasts.Active().Msg, "delete the DAC instead")
	assert.Equal(t, toastWarn, m.toasts.Active().Sev, "a refusal is the check working: it fades")
}

// A deferred orphan delete is a refusal too; a failed check is an error
// that names the retry key.
//
//nolint:paralleltest // swaps the package-level plan seam
func TestCtrlX_DeferredWarnsFailureRetries(t *testing.T) {
	for _, tc := range []struct {
		err  error
		sev  toastSeverity
		next string
	}{
		{k8s.ErrDeleteDeferred, toastWarn, ""},
		{errors.New("check DAC d1 (ome.io): forbidden"), toastError, "^x retry"},
	} {
		m := newInferenceServiceModel(t)
		stubOrphanPlan(t, "", tc.err)
		cmd := m.handleItemActions(ctrlX())
		m.handleISVCDeletePlannedMsg(cmd().(isvcDeletePlannedMsg))
		require.NotNil(t, m.toasts.Active())
		assert.Equal(t, tc.sev, m.toasts.Active().Sev, tc.err)
		assert.Equal(t, tc.next, m.toasts.Active().Next, tc.err)
	}
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
	_, cmd := m.updateConfirmView(keyPress("y"))
	assert.Nil(t, cmd, "irreversible: lowercase y doesn't confirm")

	_, cmd = m.updateConfirmView(keyPress("Y"))
	require.NotNil(t, cmd, "confirming must dispatch the delete")
	msg, ok := actionResult(t, cmd).(isvcDeleteResultMsg)
	require.True(t, ok)
	assert.Equal(t, "ns1", gotNS)
	assert.Equal(t, "svc1", gotName)

	m.handleISVCDeleteResultMsg(msg)
	require.NotNil(t, m.toasts.Active())
	assert.Equal(t, "force-deleted ns1/svc1: finalizer removed", m.toasts.Active().Msg)

	m.handleISVCDeleteResultMsg(isvcDeleteResultMsg{key: msg.key, err: errors.New("boom"), env: m.environment})
	assert.Contains(t, m.toasts.Active().Msg, "delete ns1/svc1 failed: boom")
}
