package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Sort GPUs moved to s g, freeing G: it goes to the last row, and g (the
// table's own key) to the first.
func TestGoToEnds_AndSortByPrefix(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	m.category = domain.GPUPool
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(m.category, common.ListView)
	m.updateColumns()
	m.updateRows(true)
	m.table.SetCursor(1)

	_, _ = m.Update(keyPress("s"))
	_, _ = m.Update(keyPress("g"))
	assert.Equal(t, "GPUs", m.sortColumn)
	assert.Equal(t, 1, m.table.Cursor(), "sorting must not move the cursor")

	_, _ = m.Update(keyPress("G"))
	assert.Equal(t, 3, m.table.Cursor())
	_, _ = m.Update(keyPress("g"))
	assert.Equal(t, 0, m.table.Cursor())
	_, _ = m.Update(keyPress("end"))
	assert.Equal(t, 3, m.table.Cursor())
}

// esc in the filter input closes it but keeps the drill-in scope.
func TestEscInFilterInput_KeepsScope(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	scope := &domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"}
	m.scope = scope
	m.viewMode = common.ListView

	_, _ = m.Update(keyPress("/"))
	require.Equal(t, common.EditInput, m.inputMode)
	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Equal(t, scope, m.scope)

	_, _ = m.Update(keyPress("esc"))
	assert.Equal(t, domain.DedicatedAICluster, m.category, "a second esc returns to the parent list")
	assert.Equal(t, scope, m.scope, "the scope stays, so the cursor lands on the row drilled in from")
}

// esc in the filter input still clears an applied filter.
func TestEscInFilterInput_ClearsAppliedFilter(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress("/"))
	m.filter = "isvc"
	m.textInput.SetValue("isvc")

	_, cmd := m.Update(keyPress("esc"))
	assert.Equal(t, common.NormalInput, m.inputMode)
	assert.Empty(t, m.textInput.Value())
	assert.NotNil(t, cmd, "clearing the filter re-filters the table")
}

// A backtick typed into the filter or alias input is text, not the log
// toggle; in normal input it still opens the log.
func TestBacktickInInput_IsText(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress("/"))
	_, _ = m.Update(keyPress("`"))
	assert.Equal(t, common.ListView, m.viewMode, "filter input")
	assert.Equal(t, "`", m.textInput.Value(), "filter input")

	m = newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress(":"))
	_, _ = m.Update(keyPress("`"))
	assert.Equal(t, common.PaletteView, m.viewMode, "palette query")
	assert.Equal(t, "`", m.palette.Query(), "palette query")

	m = newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress("`"))
	assert.Equal(t, common.LogView, m.viewMode)
}
