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
		Request: confirm.Request{Action: mutation.DrainNode, Target: "gpu-1"},
		run:     func(int) tea.Cmd { return nil },
	})

	assert.Nil(t, cmd, "opening the modal issues no command")
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, common.ListView, m.confirmPending.returnView)
	assert.Equal(t, "Drain", m.confirm.Request().Verb())
}

func TestDismissConfirm_RestoresViewAndClears(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	m.requestConfirm(confirmRequest{Request: confirm.Request{Action: mutation.DrainNode}, run: func(int) tea.Cmd { return nil }})

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
		Request: confirm.Request{Action: act, Target: "dac-1"},
		run:     func(int) tea.Cmd { ran = true; return nil },
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
	assert.Equal(t, mutation.Irreversible, dac.Action.Tier)
	assert.Equal(t, "Delete", dac.Verb())
	assert.Equal(t, "DAC", dac.Noun())
	assert.Equal(t, itemKeyString(key), dac.Target)
	assert.NotEmpty(t, dac.Warning)
	assert.NotNil(t, dac.run)

	m.category = domain.GPUNode
	node := m.confirmDelete(key, "")
	assert.Equal(t, mutation.Irreversible, node.Action.Tier)
	assert.Equal(t, "Terminate", node.Verb())
	assert.Equal(t, "node", node.Noun())

	m.category = domain.InferenceService
	isvc := m.confirmDelete(key, k8s.OrphanDeleted)
	assert.Equal(t, mutation.Irreversible, isvc.Action.Tier)
	assert.Equal(t, "Delete", isvc.Verb())
	assert.Equal(t, "orphaned InferenceService", isvc.Noun())
	assert.Equal(t, "Force-delete", m.confirmDelete(key, k8s.OrphanFinalized).Verb())
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
			assert.Equal(t, mutation.Recoverable, tc.got.Action.Tier)
			assert.Equal(t, tc.action, tc.got.Verb())
			assert.Equal(t, tc.kind, tc.got.Noun())
			if tc.name != "scale" { // its notice follows the count; see scale_confirm_test.go
				assert.NotEmpty(t, tc.got.Warning, "recoverable actions say what they do")
			}
			assert.NotNil(t, tc.got.run)
		})
	}
}

func TestConfirmView_RecoverableRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.confirm = confirm.New(confirm.Request{Action: mutation.DrainNode, Target: "gpu-1"})
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
	m.confirm = confirm.New(confirm.Request{
		Action: mutation.TerminateNode, Target: "gpu-1", Warning: "Boot volume destroyed. Cannot undo.",
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
	m.requestConfirm(confirmRequest{Request: confirm.Request{Action: mutation.DrainNode, Target: "n1"}, run: func(int) tea.Cmd { return nil }})
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
	m.requestConfirm(confirmRequest{Request: confirm.Request{Action: mutation.DrainNode, Target: "n1"}, run: func(int) tea.Cmd { return nil }})
	assert.Contains(t, ansi.Strip(m.confirmView()), "Kind     GPUNode")
}
