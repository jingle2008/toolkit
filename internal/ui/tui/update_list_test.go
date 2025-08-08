package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestHandleSetFilterMsg(t *testing.T) {
	t.Parallel()
	ti := textinput.New()
	m := &Model{
		textInput: &ti,
	}
	msg := SetFilterMsg("foo")
	cmd := m.handleSetFilterMsg(msg)
	assert.Equal(t, "foo", m.newFilter)
	assert.Equal(t, "foo", m.textInput.Value())
	assert.NotNil(t, cmd)
}

func TestHandleSpinnerTickMsg(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		loadingSpinner: &s,
	}
	msg := spinner.TickMsg{}
	cmd := m.handleSpinnerTickMsg(msg)
	assert.NotNil(t, cmd)
}

func TestHandleNextCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
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
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
		loadingTimer:   &w,
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
	return &Model{
		history:        append([]domain.Category{}, hist...),
		historyIdx:     idx,
		loadingSpinner: &spinner.Model{},
		loadingTimer:   &stopwatch.Model{},
	}
}

func TestMoveHistoryBackForward(t *testing.T) {
	t.Parallel()
	hist := []domain.Category{domain.Tenant, domain.GpuPool, domain.GpuNode}
	m := makeModelWithHistory(hist, 2)
	// Move back
	cmd := m.moveHistory(-1)
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.historyIdx)
	assert.Equal(t, domain.GpuPool, m.history[m.historyIdx])
	// Move forward
	cmd = m.moveHistory(1)
	assert.NotNil(t, cmd)
	assert.Equal(t, 2, m.historyIdx)
	assert.Equal(t, domain.GpuNode, m.history[m.historyIdx])
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
	hist := []domain.Category{domain.Tenant, domain.GpuPool}
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
