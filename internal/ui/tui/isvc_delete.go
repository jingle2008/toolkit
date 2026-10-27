package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
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
	env     models.Environment
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
	env, kubeConfig, opCtx := m.environment, m.kubeConfig, m.opCtxFunc()
	return func() tea.Msg {
		ctx, cancel := opCtx()
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
		text := fmt.Sprintf("can't delete %s: %v", target, msg.err)
		// A refusal is the check working, not a failure: it fades.
		if errors.Is(msg.err, k8s.ErrNotOrphan) || errors.Is(msg.err, k8s.ErrDeleteDeferred) {
			return m.showToast(text, toastWarn)
		}
		return m.showErrorWithNext(text, keys.Delete.Help().Key+" retry")
	}
	if m.category != domain.InferenceService || m.viewMode != common.ListView {
		return nil
	}
	return m.requestConfirm(m.confirmDelete(msg.key, msg.outcome))
}

// deleteOrphanInferenceService runs the delete, audited as a, the action
// the plan chose; the delete re-checks the target, so a change since the
// plan (say, the DAC came back) refuses. The k8s watch removes the row.
func (m *Model) deleteOrphanInferenceService(itemKey models.ItemKey, a mutation.Action) tea.Cmd {
	svc, ok := findItem(m.dataset, domain.InferenceService, itemKey).(*models.InferenceService)
	if !ok || svc == nil {
		return m.showToast(itemKeyString(itemKey)+" is gone", toastInfo)
	}
	namespace, name := svc.Namespace, svc.Name
	env, kubeConfig, opCtx, logger := m.environment, m.kubeConfig, m.opCtxFunc(), m.logger
	return m.trackAction(strings.ToLower(actionVerb(a))+" "+namespace+"/"+name, "", itemKey, func(func(string)) tea.Msg {
		ctx, cancel := opCtx()
		defer cancel()
		var outcome k8s.OrphanDeleteOutcome
		err := audited(logger, a, namespace+"/"+name, func() error {
			var err error
			outcome, err = deleteOrphanISVCFn(ctx, kubeConfig, env.KubeContext(), namespace, name)
			return err
		})
		return isvcDeleteResultMsg{key: itemKey, outcome: outcome, err: err, env: env}
	})
}

func (m *Model) handleISVCDeleteResultMsg(msg isvcDeleteResultMsg) tea.Cmd {
	target := itemKeyString(msg.key) + m.envNote(msg.env)
	if msg.err != nil {
		m.logger.Error("failed to delete inference service", "key", msg.key, "error", msg.err)
		return m.showErrorWithNext(fmt.Sprintf("delete %s failed: %v", target, msg.err), keys.Delete.Help().Key+" retry")
	}
	return m.showToast(msg.outcome.Summary(target), toastInfo)
}
