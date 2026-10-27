package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/confirm"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestConfirmView_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Confirm", common.ConfirmView.String())
}

func newConfirmTestModel(t *testing.T) *Model {
	t.Helper()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	require.NoError(t, err)
	return m
}

func TestRequestConfirm_OpensModalAndCapturesReturnView(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView

	cmd := m.requestConfirm(confirmRequest{
		action: mutation.DrainNode, target: "gpu-1",
		run: func(int) tea.Cmd { return nil },
	})

	assert.Nil(t, cmd, "opening the modal issues no command")
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, common.ListView, m.confirmPending.returnView)
	assert.Equal(t, "Drain", actionVerb(m.confirmPending.action))
}

func TestDismissConfirm_RestoresViewAndClears(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	m.requestConfirm(confirmRequest{action: mutation.DrainNode, run: func(int) tea.Cmd { return nil }})

	m.dismissConfirm()

	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, confirm.Model{}, m.confirm, "overlay must be cleared")
	assert.Equal(t, confirmPending{}, m.confirmPending, "pending action must be cleared")
}

// armConfirm puts the model into ConfirmView with a run thunk that records
// whether it fired, returning a pointer to that flag.
func armConfirm(m *Model, tier mutation.Tier) *bool {
	ran := false
	act := mutation.DrainNode // a recoverable action
	if tier == mutation.Irreversible {
		act = mutation.DeleteDAC
	}
	m.viewMode = common.ListView
	m.requestConfirm(confirmRequest{
		action: act, target: "dac-1",
		run: func(int) tea.Cmd { ran = true; return nil },
	})
	return &ran
}

func TestUpdateConfirmView_Recoverable(t *testing.T) {
	t.Parallel()
	t.Run("y confirms and dismisses", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("y"))
		assert.True(t, *ran, "y must run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("n cancels without running", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("n"))
		assert.False(t, *ran, "n must not run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("esc cancels", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("esc"))
		assert.False(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("unrelated key is swallowed", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Recoverable)
		_, _ = m.updateConfirmView(keyPress("x"))
		assert.False(t, *ran)
		assert.Equal(t, common.ConfirmView, m.viewMode, "stays in modal")
	})
}

func TestUpdateConfirmView_Irreversible(t *testing.T) {
	t.Parallel()
	t.Run("capital Y confirms", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Irreversible)
		_, _ = m.updateConfirmView(keyPress("Y"))
		assert.True(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("lowercase y asks for capital Y (neither runs nor cancels)", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Irreversible)
		_, _ = m.updateConfirmView(keyPress("y"))
		assert.False(t, *ran, "lowercase y must not run an irreversible action")
		assert.Equal(t, common.ConfirmView, m.viewMode, "lowercase y keeps the modal open")
		assert.True(t, m.confirm.Nudged(), "lowercase y prompts for capital Y")
		_, _ = m.updateConfirmView(keyPress("Y"))
		assert.True(t, *ran, "capital Y still confirms after the nudge")
	})
}

func TestUpdateConfirmView_CtrlCQuits(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	armConfirm(m, mutation.Irreversible)
	_, cmd := m.updateConfirmView(keyPress("ctrl+c"))
	require.NotNil(t, cmd, "ctrl+c must return a command (tea.Quit)")
}

func TestConfirmDelete_TierByCategory(t *testing.T) {
	t.Parallel()
	key := models.ScopedItemKey{Scope: "tenant1", Name: "dac-1"}

	m := newConfirmTestModel(t)
	m.category = domain.DedicatedAICluster
	dac := m.confirmDelete(key, "")
	assert.Equal(t, mutation.Irreversible, dac.action.Tier)
	assert.Equal(t, "Delete", actionVerb(dac.action))
	assert.Equal(t, "DAC", actionNoun(dac.action))
	assert.Equal(t, itemKeyString(key), dac.target)
	assert.NotEmpty(t, dac.warning)
	assert.NotNil(t, dac.run)

	m.category = domain.GPUNode
	node := m.confirmDelete(key, "")
	assert.Equal(t, mutation.Irreversible, node.action.Tier)
	assert.Equal(t, "Terminate", actionVerb(node.action))
	assert.Equal(t, "node", actionNoun(node.action))

	m.category = domain.InferenceService
	isvc := m.confirmDelete(key, k8s.OrphanDeleted)
	assert.Equal(t, mutation.Irreversible, isvc.action.Tier)
	assert.Equal(t, "Delete", actionVerb(isvc.action))
	assert.Equal(t, "orphaned InferenceService", actionNoun(isvc.action))
	assert.Equal(t, "Force-delete", actionVerb(m.confirmDelete(key, k8s.OrphanFinalized).action))
}

func TestConfirmRecoverableBuilders(t *testing.T) {
	t.Parallel()
	key := models.ScopedItemKey{Scope: "pool1", Name: "gpu-1"}
	m := newConfirmTestModel(t)
	m.category = domain.GPUNode

	for _, tc := range []struct {
		name   string
		got    confirmRequest
		action string
		kind   string
	}{
		{"cordon", m.confirmCordon(key, &models.GPUNode{Name: "gpu-1"}), "Cordon", "node"},
		{"uncordon", m.confirmCordon(key, &models.GPUNode{Name: "gpu-1", IsSchedulingDisabled: true}), "Uncordon", "node"},
		{"drain", m.confirmDrain(key), "Drain", "node"},
		{"restart", m.confirmRestart(key, nil), "Restart", "node"},
		{"scale", m.confirmScale(key, models.GPUPool{Name: "gpu-1", ID: "p", Size: 4, ActualSize: 2}), "Scale up", "GPU pool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mutation.Recoverable, tc.got.action.Tier)
			assert.Equal(t, tc.action, actionVerb(tc.got.action))
			assert.Equal(t, tc.kind, actionNoun(tc.got.action))
			if tc.name != "scale" { // its notice follows the count; see scale_confirm_test.go
				assert.NotEmpty(t, tc.got.warning, "recoverable actions say what they do")
			}
			assert.NotNil(t, tc.got.run)
		})
	}
}

func TestConfirmView_RecoverableRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.requestConfirm(confirmRequest{action: mutation.DrainNode, target: "gpu-1"})
	out := ansi.Strip(m.confirmView())
	assert.Contains(t, out, "Drain node")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "y confirm")
	assert.Contains(t, out, "[ Cancel ]")
	assert.NotContains(t, out, "⚠")
}

func TestConfirmView_IrreversibleRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.requestConfirm(confirmRequest{
		action: mutation.TerminateNode, target: "gpu-1", warning: "Boot volume destroyed. Cannot undo.",
	})
	out := ansi.Strip(m.confirmView())
	assert.Contains(t, out, "⚠ Terminate node")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "Boot volume destroyed. Cannot undo.")
	assert.Contains(t, out, "Y (shift+y) confirm · n/esc cancel")
	assert.Contains(t, out, "[ Terminate ]")
}

func TestConfirmView_ProductionContext(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.updateLayout(120, 40)
	m.requestConfirm(confirmRequest{action: mutation.DrainNode, target: "n1", run: func(int) tea.Cmd { return nil }})
	assert.NotContains(t, ansi.Strip(m.confirmView()), "PRODUCTION")

	m.environment.Type = "prod"
	assert.Contains(t, ansi.Strip(m.confirmView()), "PRODUCTION dp-prod-phx")
}

// The Kind row names the category acted on, as everywhere else does.
func TestConfirmView_KindIsCategory(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.updateLayout(120, 40)
	m.category = domain.GPUNode
	m.requestConfirm(confirmRequest{action: mutation.DrainNode, target: "n1", run: func(int) tea.Cmd { return nil }})
	assert.Contains(t, ansi.Strip(m.confirmView()), "Kind     GPUNode")
}

// The modal names every action it can confirm — all but set tenant,
// which the edit-tenant form confirms — and a label names one action.
func TestActionLabels_CoverModalActions(t *testing.T) {
	t.Parallel()
	for _, a := range mutation.Actions() {
		if a == mutation.SetTenant {
			continue
		}
		assert.NotEmpty(t, actionVerb(a), a.String())
		assert.NotEmpty(t, actionNoun(a), a.String())
	}
	assert.NotContains(t, actionLabels, mutation.SetTenant)
}

// The modal's tier is the action's mutation tier.
func TestConfirmTier(t *testing.T) {
	t.Parallel()
	assert.Equal(t, confirm.Recoverable, confirmTier(mutation.DrainNode))
	assert.Equal(t, confirm.Irreversible, confirmTier(mutation.DeleteDAC))

	m := newConfirmTestModel(t)
	m.requestConfirm(confirmRequest{action: mutation.DeleteDAC, target: "dac-1"})
	assert.Equal(t, confirm.Irreversible, m.confirm.Spec().Tier)
	assert.Equal(t, "Delete DAC", m.confirm.Spec().Title)
}

// scaleCount is the one rule the scale field's Check and the confirmed
// run share.
func TestScaleCount(t *testing.T) {
	t.Parallel()
	pool := models.GPUPool{Name: "p", ID: "x", Size: 4, ActualSize: 2}
	n, problem := scaleCount(pool, " 3 ")
	assert.Equal(t, 3, n)
	assert.Empty(t, problem)
	for _, text := range []string{"", "x", "0", "101"} {
		n, problem = scaleCount(pool, text)
		assert.Zero(t, n, text)
		assert.NotEmpty(t, problem, text)
	}

	f := scaleField(pool)
	assert.Equal(t, "2", f.Initial, "the pool's default count")
	notice, problem := f.Check("2")
	assert.Equal(t, mutation.PoolScaleNotice(pool, 2), notice)
	assert.Empty(t, problem)
	notice, problem = f.Check("")
	assert.Empty(t, notice)
	assert.Equal(t, "Enter a whole number of instances, at least 1.", problem)
}

// A confirm whose text the parent's count rule rejects runs nothing and
// leaves the modal open; the dialog's Check prevents it reaching here.
func TestAcceptConfirm_InvalidCountRunsNothing(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	ran := false
	pool := models.GPUPool{Name: "p", ID: "x", Size: 4, ActualSize: 2}
	c := m.confirmScale(models.ScopedItemKey{Name: "p"}, pool)
	c.run = func(int) tea.Cmd { ran = true; return nil }
	m.requestConfirm(c)
	assert.Nil(t, m.acceptConfirm("0"))
	assert.False(t, ran)
	assert.Equal(t, common.ConfirmView, m.viewMode)
}
