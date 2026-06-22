package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
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
		tier:   tierRecoverable,
		action: "Drain",
		kind:   "node",
		target: "gpu-1",
		run:    func() tea.Cmd { return nil },
	})

	assert.Nil(t, cmd, "opening the modal issues no command")
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, common.ListView, m.confirm.returnView)
	assert.Equal(t, "Drain", m.confirm.action)
}

func TestDismissConfirm_RestoresViewAndClears(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	m.requestConfirm(confirmOverlay{tier: tierRecoverable, action: "Drain", run: func() tea.Cmd { return nil }})

	m.dismissConfirm()

	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, confirmOverlay{}, m.confirm, "overlay must be cleared")
}

// armConfirm puts the model into ConfirmView with a run thunk that records
// whether it fired, returning a pointer to that flag.
func armConfirm(m *Model, tier confirmTier) *bool {
	ran := false
	m.confirm = confirmOverlay{
		tier:       tier,
		action:     "Delete",
		kind:       "DAC",
		target:     "dac-1",
		returnView: common.ListView,
		run:        func() tea.Cmd { ran = true; return nil },
	}
	m.viewMode = common.ConfirmView
	return &ran
}

func keyMsg(s string) tea.KeyMsg {
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestUpdateConfirmView_Recoverable(t *testing.T) {
	t.Parallel()
	t.Run("y confirms and dismisses", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("y"))
		assert.True(t, *ran, "y must run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("n cancels without running", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("n"))
		assert.False(t, *ran, "n must not run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("esc cancels", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("esc"))
		assert.False(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("unrelated key is swallowed", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("x"))
		assert.False(t, *ran)
		assert.Equal(t, common.ConfirmView, m.viewMode, "stays in modal")
	})
}

func TestUpdateConfirmView_Irreversible(t *testing.T) {
	t.Parallel()
	t.Run("capital Y confirms", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierIrreversible)
		_, _ = m.updateConfirmView(keyMsg("Y"))
		assert.True(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("lowercase y cancels (does not run)", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierIrreversible)
		_, _ = m.updateConfirmView(keyMsg("y"))
		assert.False(t, *ran, "lowercase y must not run an irreversible action")
		assert.Equal(t, common.ListView, m.viewMode, "lowercase y cancels the modal")
	})
}

func TestUpdateConfirmView_CtrlCQuits(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	armConfirm(m, tierIrreversible)
	_, cmd := m.updateConfirmView(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "ctrl+c must return a command (tea.Quit)")
}

func TestConfirmDelete_TierByCategory(t *testing.T) {
	t.Parallel()
	key := models.ScopedItemKey{Scope: "tenant1", Name: "dac-1"}

	m := newConfirmTestModel(t)
	m.category = domain.DedicatedAICluster
	dac := m.confirmDelete(key)
	assert.Equal(t, tierIrreversible, dac.tier)
	assert.Equal(t, "Delete", dac.action)
	assert.Equal(t, "DAC", dac.kind)
	assert.Equal(t, itemKeyString(key), dac.target)
	assert.NotEmpty(t, dac.warning)
	assert.NotNil(t, dac.run)

	m.category = domain.GPUNode
	node := m.confirmDelete(key)
	assert.Equal(t, tierIrreversible, node.tier)
	assert.Equal(t, "Terminate", node.action)
	assert.Equal(t, "node", node.kind)
}

func TestConfirmRecoverableBuilders(t *testing.T) {
	t.Parallel()
	key := models.ScopedItemKey{Scope: "pool1", Name: "gpu-1"}
	m := newConfirmTestModel(t)

	for _, tc := range []struct {
		name   string
		got    confirmOverlay
		action string
		kind   string
	}{
		{"cordon", m.confirmCordon(key), "Toggle cordon", "node"},
		{"drain", m.confirmDrain(key), "Drain", "node"},
		{"reboot", m.confirmReboot(key), "Reboot", "node"},
		{"scale", m.confirmScale(key), "Scale up", "GPU pool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tierRecoverable, tc.got.tier)
			assert.Equal(t, tc.action, tc.got.action)
			assert.Equal(t, tc.kind, tc.got.kind)
			assert.Empty(t, tc.got.warning, "recoverable actions carry no warning line")
			assert.NotNil(t, tc.got.run)
		})
	}
}
