package tui

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	m.filterGen = 2

	m.handleFilterApplyMsg(filterApplyMsg{Value: "new", Gen: 1})
	if m.filter != "old" {
		t.Fatalf("unexpected filter update on stale gen: %q", m.filter)
	}

	m.handleFilterApplyMsg(filterApplyMsg{Value: "new", Gen: 2})
	if m.filter != "new" {
		t.Fatalf("expected filter to update, got %q", m.filter)
	}
}

func TestHandleDeleteErrMsg_GPUNodeRestoresStatus(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUNode
	node := models.GPUNode{Name: "node1", NodePool: "pool1"}
	node.SetStatus("Deleting")
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{
		"pool1": {node},
	}
	m.updateColumns()
	m.updateRows(false)

	msg := deleteErrMsg{
		err:      errors.New("boom"),
		category: domain.GPUNode,
		key:      models.ScopedItemKey{Scope: "pool1", Name: "node1"},
	}
	msg.prevState = "OK"
	m.handleDeleteErrMsg(msg)

	got := m.dataset.GPUNodeMap["pool1"][0].GetStatus()
	if got != "OK" {
		t.Fatalf("expected status OK, got %q", got)
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
	m.history = []domain.Category{m.category}
	m.historyIdx = 0
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
	m.history = []domain.Category{m.category}
	m.historyIdx = 0
	cmd := m.handlePrevCategory()
	assert.NotNil(t, cmd)
}

// --- History/Navigation Tests ---

func TestSeedHistory(t *testing.T) {
	t.Parallel()
	m := &Model{
		history:    []domain.Category{},
		historyIdx: -1,
		category:   domain.Tenant,
	}
	// Simulate initial seeding as in NewModel
	if len(m.history) == 0 {
		m.history = []domain.Category{m.category}
		m.historyIdx = 0
	}
	assert.Len(t, m.history, 1)
	assert.Equal(t, 0, m.historyIdx)
	assert.Equal(t, domain.Tenant, m.history[0])
}

func TestHistoryCap20(t *testing.T) {
	t.Parallel()
	m := &Model{
		history:    []domain.Category{},
		historyIdx: -1,
	}
	// Add 25 categories, should cap at 20
	for i := range 25 {
		m.pushHistory(domain.Category(i))
	}
	assert.Len(t, m.history, 20)
	assert.Equal(t, 19, m.historyIdx)
	assert.Equal(t, domain.Category(5), m.history[0])
	assert.Equal(t, domain.Category(24), m.history[19])
}

func makeModelWithHistory(hist []domain.Category, idx int) *Model {
	tbl := table.New()
	ti := textinput.New()
	return &Model{
		history:        append([]domain.Category{}, hist...),
		historyIdx:     idx,
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
	assert.Equal(t, 1, m.historyIdx)
	assert.Equal(t, domain.GPUPool, m.history[m.historyIdx])
	// Move forward
	cmd = m.moveHistory(1)
	assert.NotNil(t, cmd)
	assert.Equal(t, 2, m.historyIdx)
	assert.Equal(t, domain.GPUNode, m.history[m.historyIdx])
	// Out of bounds (forward)
	cmd = m.moveHistory(1)
	assert.Nil(t, cmd)
	assert.Equal(t, 2, m.historyIdx)
	// Out of bounds (back)
	m.historyIdx = 0
	cmd = m.moveHistory(-1)
	assert.Nil(t, cmd)
	assert.Equal(t, 0, m.historyIdx)
}

func TestBackForwardKeyBindings(t *testing.T) {
	t.Parallel()
	hist := []domain.Category{domain.Tenant, domain.GPUPool}
	m := makeModelWithHistory(hist, 1)
	// Simulate "[" key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}
	cmds := m.handleNormalKeys(msg)
	assert.Equal(t, 0, m.historyIdx)
	assert.NotNil(t, cmds)
	// Simulate "]" key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}
	cmds = m.handleNormalKeys(msg)
	assert.Equal(t, 1, m.historyIdx)
	assert.NotNil(t, cmds)
}
