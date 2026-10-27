package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/tenantform"
)

// Charm v2 delivers a terminal paste (cmd+v) as tea.PasteMsg, not a key
// press: every text input must take it.

func paste(s string) tea.PasteMsg { return tea.PasteMsg{Content: s} }

// A paste fills the focused tenant-form field and clears Name's
// "required" mark, as typing does.
func TestPaste_EditTenantForm(t *testing.T) {
	t.Parallel()
	m := openForm(t)
	m.viewMode = common.EditTenantView
	_, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}) // save without a name
	require.Equal(t, "required", m.editTenant.NameErr())
	require.Equal(t, tenantform.FieldName, m.editTenant.Focus())

	_, _ = m.Update(paste("acme"))
	assert.Equal(t, "acme", m.editTenant.Name())
	assert.Empty(t, m.editTenant.NameErr())

	m.editTenant = m.editTenant.FocusOn(tenantform.FieldNote)
	_, _ = m.Update(paste("shared tenancy"))
	assert.Equal(t, "shared tenancy", m.editTenant.Note())
	assert.Equal(t, "acme", m.editTenant.Name(), "only the focused field")
}

// A paste into the filter input sets the text and debounces the filter;
// with the input closed it does nothing.
func TestPaste_ListFilter(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(paste("isvc"))
	assert.Empty(t, m.textInput.Value(), "input closed")

	_, _ = m.Update(keyPress("/"))
	require.Equal(t, common.EditInput, m.inputMode)
	_, cmd := m.Update(paste("isvc"))
	assert.Equal(t, "isvc", m.textInput.Value())
	assert.NotNil(t, cmd, "the filter is debounced")
}

func TestPaste_PaletteQuery(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress(":"))
	require.Equal(t, common.PaletteView, m.viewMode)
	_, _ = m.Update(paste("dac"))
	assert.Equal(t, "dac", m.palette.Query())
}

func TestPaste_LogFilter(t *testing.T) {
	t.Parallel()
	m := newGoToModel(t)
	m.viewMode = common.ListView
	_, _ = m.Update(keyPress("`"))
	require.Equal(t, common.LogView, m.viewMode)
	_, _ = m.Update(keyPress("/"))
	require.True(t, m.log.typing)
	_, _ = m.Update(paste(" watch closed "))
	assert.Equal(t, "watch closed", m.log.filter)
}
