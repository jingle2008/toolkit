package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the orphan check and the delete.
var (
	planOrphanISVCDeleteFn = k8s.PlanOrphanInferenceServiceDelete
	deleteOrphanISVCFn     = k8s.DeleteOrphanInferenceService
)

// isvcDeletePlannedMsg carries the orphan check for the endpoint at key:
// what deleting it would do, or why it's refused.
type isvcDeletePlannedMsg struct {
	key     models.ItemKey
	outcome k8s.OrphanDeleteOutcome
	err     error
	env     models.Environment
}

type isvcDeleteResultMsg struct {
	key     models.ItemKey
	outcome k8s.OrphanDeleteOutcome
	err     error
}

// planDeleteInferenceService starts ctrl+x on an InferenceService. Only an
// orphan (its DAC is gone) can be deleted, and whether a delete or a
// force-delete applies depends on the cluster, so the check runs first
// and the confirm modal opens with its answer.
func (m *Model) planDeleteInferenceService(itemKey models.ItemKey, item any) tea.Cmd {
	svc, ok := item.(*models.InferenceService)
	if !ok || svc == nil {
		return nil
	}
	namespace, name := svc.Namespace, svc.Name
	env, kubeConfig := m.environment, m.kubeConfig
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		outcome, err := planOrphanISVCDeleteFn(ctx, kubeConfig, env.KubeContext(), namespace, name)
		return isvcDeletePlannedMsg{key: itemKey, outcome: outcome, err: err, env: env}
	}
}

// handleISVCDeletePlannedMsg opens the confirm modal for an orphan, or
// toasts the refusal. A plan that lands after the user moved on — another
// view, category, or environment — is dropped rather than popping a modal
// that would act on whatever now has that key.
func (m *Model) handleISVCDeletePlannedMsg(msg isvcDeletePlannedMsg) tea.Cmd {
	if !m.inCurrentEnv(msg.env) {
		return nil
	}
	target := itemKeyString(msg.key)
	if msg.err != nil {
		return m.showToast(fmt.Sprintf("can't delete %s: %v", target, msg.err), toastError)
	}
	if m.category != domain.InferenceService || m.viewMode != common.ListView {
		return nil
	}
	return m.requestConfirm(m.confirmDelete(msg.key, msg.outcome))
}

// deleteOrphanInferenceService runs the delete; the delete re-checks the
// target, so a change since the plan (say, the DAC came back) refuses.
// The k8s watch removes the row.
func (m *Model) deleteOrphanInferenceService(itemKey models.ItemKey) tea.Cmd {
	svc, ok := findItem(m.dataset, domain.InferenceService, itemKey).(*models.InferenceService)
	if !ok || svc == nil {
		return m.showToast(itemKeyString(itemKey)+" is gone", toastInfo)
	}
	m.logger.Infow("action started", "action", "deleteOrphanInferenceService", "service", itemKeyString(itemKey))
	namespace, name := svc.Namespace, svc.Name
	kubeConfig, kubeContext := m.kubeConfig, m.environment.KubeContext()
	return func() tea.Msg {
		ctx, cancel := m.opCtx()
		defer cancel()
		outcome, err := deleteOrphanISVCFn(ctx, kubeConfig, kubeContext, namespace, name)
		return isvcDeleteResultMsg{key: itemKey, outcome: outcome, err: err}
	}
}

func (m *Model) handleISVCDeleteResultMsg(msg isvcDeleteResultMsg) tea.Cmd {
	target := itemKeyString(msg.key)
	if msg.err != nil {
		m.logger.Errorw("failed to delete inference service", "key", msg.key, "error", msg.err)
		return m.showToast(fmt.Sprintf("delete %s failed: %v", target, msg.err), toastError)
	}
	return m.showToast(msg.outcome.Summary(target), toastInfo)
}
