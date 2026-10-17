package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	return m
}

func TestRequestConfirm_OpensModalAndCapturesReturnView(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView

	cmd := m.requestConfirm(confirmOverlay{
		act:    mutation.DrainNode,
		target: "gpu-1",
		run:    func() tea.Cmd { return nil },
	})

	assert.Nil(t, cmd, "opening the modal issues no command")
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, common.ListView, m.confirm.returnView)
	assert.Equal(t, "Drain", m.confirm.verb())
}

func TestDismissConfirm_RestoresViewAndClears(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	m.requestConfirm(confirmOverlay{act: mutation.DrainNode, run: func() tea.Cmd { return nil }})

	m.dismissConfirm()

	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, confirmOverlay{}, m.confirm, "overlay must be cleared")
}

// armConfirm puts the model into ConfirmView with a run thunk that records
// whether it fired, returning a pointer to that flag.
func armConfirm(m *Model, tier mutation.Tier) *bool {
	ran := false
	act := mutation.DrainNode // a recoverable action
	if tier == mutation.Irreversible {
		act = mutation.DeleteDAC
	}
	m.confirm = confirmOverlay{
		act:        act,
		target:     "dac-1",
		returnView: common.ListView,
		run:        func() tea.Cmd { ran = true; return nil },
	}
	m.viewMode = common.ConfirmView
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
	t.Run("lowercase y cancels (does not run)", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, mutation.Irreversible)
		_, _ = m.updateConfirmView(keyPress("y"))
		assert.False(t, *ran, "lowercase y must not run an irreversible action")
		assert.Equal(t, common.ListView, m.viewMode, "lowercase y cancels the modal")
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
	assert.Equal(t, mutation.Irreversible, dac.act.Tier)
	assert.Equal(t, "Delete", dac.verb())
	assert.Equal(t, "DAC", dac.noun())
	assert.Equal(t, itemKeyString(key), dac.target)
	assert.NotEmpty(t, dac.warning)
	assert.NotNil(t, dac.run)

	m.category = domain.GPUNode
	node := m.confirmDelete(key, "")
	assert.Equal(t, mutation.Irreversible, node.act.Tier)
	assert.Equal(t, "Terminate", node.verb())
	assert.Equal(t, "node", node.noun())

	m.category = domain.InferenceService
	isvc := m.confirmDelete(key, k8s.OrphanDeleted)
	assert.Equal(t, mutation.Irreversible, isvc.act.Tier)
	assert.Equal(t, "Delete", isvc.verb())
	assert.Equal(t, "orphaned InferenceService", isvc.noun())
	assert.Equal(t, "Force-delete", m.confirmDelete(key, k8s.OrphanFinalized).verb())
}

func TestConfirmRecoverableBuilders(t *testing.T) {
	t.Parallel()
	key := models.ScopedItemKey{Scope: "pool1", Name: "gpu-1"}
	m := newConfirmTestModel(t)
	m.category = domain.GPUNode

	for _, tc := range []struct {
		name   string
		got    confirmOverlay
		action string
		kind   string
	}{
		{"cordon", m.confirmCordon(key, &models.GPUNode{Name: "gpu-1"}), "Cordon", "node"},
		{"uncordon", m.confirmCordon(key, &models.GPUNode{Name: "gpu-1", IsSchedulingDisabled: true}), "Uncordon", "node"},
		{"drain", m.confirmDrain(key), "Drain", "node"},
		{"restart", m.confirmRestart(key, nil), "Restart", "node"},
		{"scale", m.confirmScale(key), "Scale up", "GPU pool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, mutation.Recoverable, tc.got.act.Tier)
			assert.Equal(t, tc.action, tc.got.verb())
			assert.Equal(t, tc.kind, tc.got.noun())
			assert.Empty(t, tc.got.warning, "recoverable actions carry no warning line")
			assert.NotNil(t, tc.got.run)
		})
	}
}

func TestConfirmView_RecoverableRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.confirm = confirmOverlay{act: mutation.DrainNode, target: "gpu-1"}
	out := m.confirmView()
	assert.Contains(t, out, "Drain")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "[y]")
	assert.NotContains(t, out, "DESTRUCTIVE")
}

func TestConfirmView_IrreversibleRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.confirm = confirmOverlay{
		act: mutation.TerminateNode, target: "gpu-1", warning: "Boot volume destroyed. Cannot undo.",
	}
	out := m.confirmView()
	assert.Contains(t, out, "DESTRUCTIVE")
	assert.Contains(t, out, "Terminate")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "Boot volume destroyed. Cannot undo.")
	assert.Contains(t, out, "Press Y")
}

// The modal names every action it can confirm — all but set tenant,
// which the edit-tenant form confirms — and a label names one action.
func TestConfirmLabels_CoverModalActions(t *testing.T) {
	t.Parallel()
	for _, a := range mutation.Actions() {
		if a == mutation.SetTenant {
			continue
		}
		l, ok := confirmLabels[a]
		if assert.True(t, ok, "no modal label for %s", a) {
			assert.NotEmpty(t, l.verb, a.String())
			assert.NotEmpty(t, l.noun, a.String())
		}
	}
	assert.NotContains(t, confirmLabels, mutation.SetTenant)
}
