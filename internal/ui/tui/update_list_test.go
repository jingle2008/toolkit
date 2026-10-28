package tui

import (
	"errors"
	"testing"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleSetFilterMsg(t *testing.T) {
	t.Parallel()
	ti := textinput.New()
	m := &Model{
		textInput: &ti,
	}
	msg := setFilterMsg("Foo")
	cmd := m.handleSetFilterMsg(msg)
	// newFilter removed; text input is updated immediately
	assert.Equal(t, "Foo", m.textInput.Value())
	// and a filterMsg is emitted with lowercased value
	requireNotNil := func(c tea.Cmd) {
		if c == nil {
			t.Fatal("expected non-nil cmd")
		}
	}
	requireNotNil(cmd)
	got := cmd()
	fm, ok := got.(filterMsg)
	if !ok {
		t.Fatalf("expected filterMsg, got %T", got)
	}
	assert.Equal(t, "foo", string(fm))
}

func TestHandleFilterApplyMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.filter = "old"
	m.gens.filter = 2

	m.handleFilterApplyMsg(filterApplyMsg{Value: "new", Gen: 1})
	if m.filter != "old" {
		t.Fatalf("unexpected filter update on stale gen: %q", m.filter)
	}

	m.handleFilterApplyMsg(filterApplyMsg{Value: "new", Gen: 2})
	if m.filter != "new" {
		t.Fatalf("expected filter to update, got %q", m.filter)
	}
}

func TestHandleDeleteErrMsg_GPUNodeToasts(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUNode
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{
		"pool1": {{Name: "node1", NodePool: "pool1"}},
	}
	m.updateColumns()
	m.updateRows(false)

	m.handleDeleteErrMsg(deleteErrMsg{
		err:      errors.New("boom"),
		category: domain.GPUNode,
		key:      models.ScopedItemKey{Scope: "pool1", Name: "node1"},
		env:      m.environment,
	})
	// The failure is shown, not only logged.
	if m.toasts.Active() == nil || m.toasts.Active().Sev != toastError || m.toasts.Active().Next != "^x retry" {
		t.Fatalf("expected an error toast naming ^x retry, got %+v", m.toasts.Active())
	}
}

func TestHandleDeleteDoneMsg_GPUNodeRemoved(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUNode
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{
		"pool1": {{Name: "node1", NodePool: "pool1"}},
	}
	m.updateColumns()
	m.updateRows(false)

	m.handleDeleteDoneMsg(deleteDoneMsg{
		category: domain.GPUNode,
		key:      models.ScopedItemKey{Scope: "pool1", Name: "node1"},
		env:      m.environment,
	})

	if len(m.dataset.GPUNodeMap["pool1"]) != 0 {
		t.Fatalf("expected node removed, got %#v", m.dataset.GPUNodeMap["pool1"])
	}
}

func TestHandleSpinnerTickMsg(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		loadingSpinner: &s,
		pendingTasks:   1, // simulate an in-flight load so the tick chain stays alive
	}
	msg := spinner.TickMsg{}
	cmd := m.handleSpinnerTickMsg(msg)
	assert.NotNil(t, cmd)
}

func TestHandleNextCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	tbl := table.New()
	ti := textinput.New()
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
		table:          &tbl,
		textInput:      &ti,
	}
	// Seed initial history as in NewModel
	m.nav.history = histOf(m.category)
	m.nav.idx = 0
	cmd := m.handleNextCategory()
	assert.NotNil(t, cmd)
}

func TestHandlePrevCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	w := stopwatch.New()
	tbl := table.New()
	ti := textinput.New()
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
		loadingTimer:   &w,
		table:          &tbl,
		textInput:      &ti,
	}
	// Seed initial history as in NewModel
	m.nav.history = histOf(m.category)
	m.nav.idx = 0
	cmd := m.handlePrevCategory()
	assert.NotNil(t, cmd)
}

// --- History/Navigation Tests ---

func TestSeedHistory(t *testing.T) {
	t.Parallel()
	m := &Model{
		nav:      navState{history: []historyEntry{}, idx: -1},
		category: domain.Tenant,
	}
	// Simulate initial seeding as in NewModel
	if len(m.nav.history) == 0 {
		m.nav.history = histOf(m.category)
		m.nav.idx = 0
	}
	assert.Len(t, m.nav.history, 1)
	assert.Equal(t, 0, m.nav.idx)
	assert.Equal(t, domain.Tenant, m.nav.history[0].category)
}

func TestHistoryCap20(t *testing.T) {
	t.Parallel()
	m := &Model{
		nav: navState{history: []historyEntry{}, idx: -1},
	}
	// Add 25 categories, should cap at 20
	for i := range 25 {
		m.pushHistory(domain.Category(i))
	}
	assert.Len(t, m.nav.history, 20)
	assert.Equal(t, 19, m.nav.idx)
	assert.Equal(t, domain.Category(5), m.nav.history[0].category)
	assert.Equal(t, domain.Category(24), m.nav.history[19].category)
}

func makeModelWithHistory(hist []domain.Category, idx int) *Model {
	tbl := table.New()
	ti := textinput.New()
	return &Model{
		nav:            navState{history: histOf(hist...), idx: idx},
		loadingSpinner: &spinner.Model{},
		loadingTimer:   &stopwatch.Model{},
		table:          &tbl,
		textInput:      &ti,
	}
}

func TestMoveHistoryBackForward(t *testing.T) {
	t.Parallel()
	hist := []domain.Category{domain.Tenant, domain.GPUPool, domain.GPUNode}
	m := makeModelWithHistory(hist, 2)
	// Move back
	cmd := m.moveHistory(-1)
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.nav.idx)
	assert.Equal(t, domain.GPUPool, m.nav.history[m.nav.idx].category)
	// Move forward
	cmd = m.moveHistory(1)
	assert.NotNil(t, cmd)
	assert.Equal(t, 2, m.nav.idx)
	assert.Equal(t, domain.GPUNode, m.nav.history[m.nav.idx].category)
	// Out of bounds (forward)
	cmd = m.moveHistory(1)
	assert.Nil(t, cmd)
	assert.Equal(t, 2, m.nav.idx)
	// Out of bounds (back)
	m.nav.idx = 0
	cmd = m.moveHistory(-1)
	assert.Nil(t, cmd)
	assert.Equal(t, 0, m.nav.idx)
}

func TestBackForwardKeyBindings(t *testing.T) {
	t.Parallel()
	hist := []domain.Category{domain.Tenant, domain.GPUPool}
	m := makeModelWithHistory(hist, 1)
	// Simulate "[" key
	msg := keyPress("[")
	cmds := m.handleNormalKeys(msg)
	assert.Equal(t, 0, m.nav.idx)
	assert.NotNil(t, cmds)
	// Simulate "]" key
	msg = keyPress("]")
	cmds = m.handleNormalKeys(msg)
	assert.Equal(t, 1, m.nav.idx)
	assert.NotNil(t, cmds)
}
