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

// openForm returns a model with the edit-tenant form open on an
// unresolved DAC row.
func openForm(t *testing.T) *Model {
	t.Helper()
	m := makeTestModel()
	m.environment.Realm = "oc1"
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc"})
	require.NotNil(t, m.editTenant, "form should open for an unresolved row")
	return m
}

func TestEditTenantView_Renders(t *testing.T) {
	t.Parallel()
	out := ansi.Strip(openForm(t).editTenantView())
	require.NotEmpty(t, out)
	assert.Contains(t, out, "abc")  // tenantID
	assert.Contains(t, out, "Name") // field label
	assert.Contains(t, out, "(•)")
}

func TestEditTenantView_NilFormIsEmpty(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.editTenant = nil
	assert.Empty(t, m.editTenantView())
}

func TestEditTenantForm_CycleFocus(t *testing.T) {
	t.Parallel()
	f := openForm(t).editTenant
	require.Equal(t, focusName, f.focus)
	f.cycleFocus(1)
	assert.Equal(t, focusInternal, f.focus)
	f.cycleFocus(1)
	assert.Equal(t, focusNote, f.focus)
	f.cycleFocus(1)
	assert.Equal(t, focusName, f.focus, "should wrap around")
	f.cycleFocus(-1)
	assert.Equal(t, focusNote, f.focus, "should wrap backward")
}

func TestHandleEditTenantKey_Flows(t *testing.T) {
	t.Parallel()
	m := openForm(t)

	// Tab advances focus to the internal toggle.
	m.handleEditTenantKey(keyPress("tab"))
	assert.Equal(t, focusInternal, m.editTenant.focus)

	// Space on the internal field flips the flag.
	was := m.editTenant.isInternal
	m.handleEditTenantKey(keyPress("space"))
	assert.NotEqual(t, was, m.editTenant.isInternal)

	// Shift+Tab returns to the name field; typed runes route to it.
	m.handleEditTenantKey(keyPress("shift+tab"))
	assert.Equal(t, focusName, m.editTenant.focus)
	m.handleEditTenantKey(keyPress("acme"))
	assert.Contains(t, m.editTenant.name.Value(), "acme")

	// Ctrl+O builds the portal-open command (not executed — it would
	// launch a real browser).
	_, cmd := m.handleEditTenantKey(keyPress("ctrl+o"))
	assert.NotNil(t, cmd)

	// Esc closes the form and returns to the list view.
	m.handleEditTenantKey(keyPress("esc"))
	assert.Nil(t, m.editTenant)
	assert.Equal(t, common.ListView, m.viewMode)
}

func TestHandleEditTenantKey_Confirm(t *testing.T) {
	t.Parallel()
	m := openForm(t)

	f := m.editTenant

	// enter moves to the next field rather than saving.
	_, cmd := m.handleEditTenantKey(keyPress("enter"))
	assert.Nil(t, cmd)
	assert.Equal(t, focusInternal, f.focus)

	// Saving without a name marks Name, focuses it, and saves nothing.
	_, cmd = m.handleEditTenantKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Nil(t, cmd)
	assert.Equal(t, "required", f.nameErr)
	assert.Equal(t, focusName, f.focus)
	assert.Contains(t, ansi.Strip(m.editTenantView()), "required")
	assert.Nil(t, m.toasts.Active(), "inline, not a toast")

	// Typing a name clears the mark; enter through to the last field saves.
	_, _ = m.handleEditTenantKey(keyPress("a"))
	assert.Empty(t, f.nameErr)
	_, _ = m.handleEditTenantKey(keyPress("enter"))
	_, _ = m.handleEditTenantKey(keyPress("enter"))
	require.Equal(t, focusNote, f.focus)
	_, cmd = m.handleEditTenantKey(keyPress("enter"))
	assert.NotNil(t, cmd, "enter on the last field saves")

	// ctrl+s saves from any field.
	f.focusOn(focusName)
	_, cmd = m.handleEditTenantKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.NotNil(t, cmd)
}

func TestModel_MetadataPath(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, makeTestModel().metadataPath())
}

// A failed save keeps the form open and names ^s as the retry.
func TestHandleTenantSaveErrMsg_NamesRetry(t *testing.T) {
	t.Parallel()
	m := openForm(t)
	m.handleTenantSaveErrMsg(tenantSaveErrMsg{err: errors.New("read-only file system")})
	require.NotNil(t, m.toasts.Active())
	assert.Equal(t, "^s retry", m.toasts.Active().Next)
	assert.NotNil(t, m.editTenant, "the form keeps the input")
}
