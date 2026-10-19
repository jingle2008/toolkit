package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// newTrackedDrain starts a drain-like tracked action on a GPU node row
// whose run never executes (the command isn't run), returning its id.
func newTrackedDrain(t *testing.T) (*Model, models.ItemKey, int) {
	t.Helper()
	m, key, _ := newGPUNodeModel(t, models.GPUNode{Name: "10.0.0.1", NodePool: "pool1", InstanceType: "BM.GPU.8"})
	m.updateColumns()
	m.updateRows(false)
	require.NotNil(t, m.trackAction("drain 10.0.0.1", "Draining", key, func(func(string)) tea.Msg { return nil }))
	require.Len(t, m.actions, 1)
	return m, key, m.actions[0].id
}

// A running action's status shows in its row and survives a reload, which
// replaces the items; it clears when the result arrives.
func TestTrackedAction_StatusSurvivesReload(t *testing.T) {
	t.Parallel()
	m, key, id := newTrackedDrain(t)
	assert.Equal(t, "Draining", statusCell(t, m))

	m.dataset.GPUNodeMap = map[string][]models.GPUNode{"pool1": {{Name: "10.0.0.1", NodePool: "pool1", InstanceType: "BM.GPU.8", IsSchedulingDisabled: true}}}
	m.updateRows(false)
	assert.Equal(t, "Draining", statusCell(t, m), "a watch reload mid-drain")

	m.Update(actionDoneMsg{id: id, msg: drainNodeResultMsg{key: key, env: m.environment}})
	assert.Empty(t, m.actions)
	assert.Equal(t, "WARN: CORDONED", statusCell(t, m))
}

// The env bar names the newest action, its latest step, how many run, and
// its environment once the user has switched away.
func TestTrackedAction_Indicator(t *testing.T) {
	t.Parallel()
	m, _, id := newTrackedDrain(t)
	assert.Contains(t, ansi.Strip(m.actionsIndicator(200)), "drain 10.0.0.1")

	m.Update(actionProgressMsg{id: id, line: "evicted pod=ns/p1"})
	assert.Contains(t, ansi.Strip(m.actionsIndicator(200)), "drain 10.0.0.1: evicted pod=ns/p1")

	m.trackAction("restart 10.0.0.2", "", "10.0.0.2", func(func(string)) tea.Msg { return nil })
	assert.Contains(t, ansi.Strip(m.actionsIndicator(200)), "2 running · restart 10.0.0.2")

	m.environment = otherEnv
	assert.Contains(t, ansi.Strip(m.actionsIndicator(200)), "restart 10.0.0.2 (dp-type-UNKNOWN)")
	assert.Contains(t, ansi.Strip(m.envBar()), "2 running")
}

// A result is handled from any view: before, results only reached the
// list view, so a drain finishing while details was open said nothing.
func TestTrackedAction_ResultFromDetailsView(t *testing.T) {
	t.Parallel()
	m, key, id := newTrackedDrain(t)
	m.viewMode = common.DetailsView
	m.Update(actionDoneMsg{id: id, msg: drainNodeResultMsg{key: key, err: errors.New("pdb"), env: m.environment}})
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, toastError, m.toasts.active.sev)
	assert.Contains(t, m.toasts.active.msg, "drain pool1/10.0.0.1 failed: pdb")
}

// Quitting with actions running asks first, listing them; without, it
// quits at once.
func TestQuit_AsksWhileActionsRun(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	cmd := m.quit()
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())

	m, _, _ = newTrackedDrain(t)
	m.viewMode = common.ListView
	assert.Nil(t, m.quit())
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Contains(t, ansi.Strip(m.confirmView()), "drain 10.0.0.1")
	assert.Contains(t, ansi.Strip(m.confirmView()), "Quit with 1 action running")

	_, cmd = m.updateConfirmView(keyPress("n"))
	assert.Nil(t, cmd)
	assert.Equal(t, common.ListView, m.viewMode, "cancel stays")

	m.quit()
	_, cmd = m.updateConfirmView(keyPress("y"))
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

// A progress report replaces a line not yet read, and reports after the
// action ends are dropped.
func TestProgressBox(t *testing.T) {
	t.Parallel()
	b := newProgressBox()
	b.report("one")
	b.report("two")
	assert.Equal(t, actionProgressMsg{id: 7, line: "two"}, b.next(7)())
	b.close()
	b.report("late")
	assert.Nil(t, b.next(7)())
}

// progressLogger reports what it logs, with the fields.
func TestProgressLogger(t *testing.T) {
	t.Parallel()
	var got []string
	l := progressLogger{Logger: newTestModel(t).logger, report: func(s string) { got = append(got, s) }}
	l.Infow("deleting endpoint", "name", "ep1")
	l.WithFields("dac", "d1").Warnw("failed to get work request status")
	assert.Equal(t, []string{"deleting endpoint name=ep1", "failed to get work request status"}, got)
}
