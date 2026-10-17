package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// G is Sort GPUs; the table no longer also takes it as go-to-bottom, so
// sorting leaves the cursor where it was. end still goes to the last row.
func TestShiftG_SortsWithoutJumpingToBottom(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	m.category = domain.GPUPool
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.updateColumns()
	m.updateRows(true)
	m.table.SetCursor(1)

	_, _ = m.Update(runeKey("G"))
	assert.Equal(t, "GPUs", m.sortColumn)
	assert.Equal(t, 1, m.table.Cursor(), "G must not also jump to the last row")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	assert.Equal(t, 3, m.table.Cursor())
}

// esc in the filter input closes it but keeps the drill-in scope.
func TestEscInFilterInput_KeepsScope(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	scope := &domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"}
	m.scope = scope
	m.viewMode = common.ListView

	_, _ = m.Update(runeKey("/"))
	require.Equal(t, common.EditInput, m.inputMode)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Equal(t, scope, m.scope)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, m.scope, "a second esc leaves the scope")
}

// esc in the filter input still clears an applied filter.
func TestEscInFilterInput_ClearsAppliedFilter(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(runeKey("/"))
	m.filter = "isvc"
	m.textInput.SetValue("isvc")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Empty(t, m.textInput.Value())
	assert.NotNil(t, cmd, "clearing the filter re-filters the table")
}

// A backtick typed into the filter or alias input is text, not the log
// toggle; in normal input it still opens the log.
func TestBacktickInInput_IsText(t *testing.T) {
	t.Parallel()
	for _, open := range []string{"/", ":"} {
		m := newGoToModel(t)
		m.viewMode = common.ListView
		_, _ = m.Update(runeKey(open))
		_, _ = m.Update(runeKey("`"))
		assert.Equalf(t, common.ListView, m.viewMode, "%s input", open)
		assert.Equalf(t, "`", m.textInput.Value(), "%s input", open)
	}

	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(runeKey("`"))
	assert.Equal(t, common.LogView, m.viewMode)
}
