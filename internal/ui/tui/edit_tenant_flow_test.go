package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/tenantform"
	"github.com/jingle2008/toolkit/pkg/models"
)

// openForm returns a model with the edit-tenant form open on an
// unresolved DAC row.
func openForm(t *testing.T) *Model {
	t.Helper()
	m := makeTestModel()
	m.environment.Realm = "oc1"
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc"})
	require.True(t, m.editTenant.Visible(), "form should open for an unresolved row")
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

func TestEditTenantView_NoFormIsEmpty(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.editTenant = tenantform.Model{}
	assert.Empty(t, m.editTenantView())
}

// With no form open the view ignores input, and the portal has no target.
func TestUpdateEditTenantView_NoForm(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	_, cmd := m.updateEditTenantView(keyPress("ctrl+s"))
	assert.Nil(t, cmd)
	assert.Nil(t, m.openPortalCmd())
}

func TestUpdateEditTenantView_Flows(t *testing.T) {
	t.Parallel()
	m := openForm(t)

	// Tab advances focus to the internal toggle.
	m.updateEditTenantView(keyPress("tab"))
	assert.Equal(t, tenantform.FieldInternal, m.editTenant.Focus())

	// Space on the internal field flips the flag.
	was := m.editTenant.Internal()
	m.updateEditTenantView(keyPress("space"))
	assert.NotEqual(t, was, m.editTenant.Internal())

	// Shift+Tab returns to the name field; typed runes route to it.
	m.updateEditTenantView(keyPress("shift+tab"))
	assert.Equal(t, tenantform.FieldName, m.editTenant.Focus())
	m.updateEditTenantView(keyPress("acme"))
	assert.Contains(t, m.editTenant.Name(), "acme")

	// Ctrl+O builds the portal-open command (not executed — it would
	// launch a real browser).
	_, cmd := m.updateEditTenantView(keyPress("ctrl+o"))
	assert.NotNil(t, cmd)

	// Esc closes the form and returns to the list view.
	m.updateEditTenantView(keyPress("esc"))
	assert.False(t, m.editTenant.Visible())
	assert.Equal(t, common.ListView, m.viewMode)
}

// ctrl+c quits from the form.
func TestUpdateEditTenantView_CtrlCQuits(t *testing.T) {
	t.Parallel()
	m := openForm(t)
	_, cmd := m.updateEditTenantView(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	assert.NotNil(t, cmd)
}

func TestUpdateEditTenantView_Confirm(t *testing.T) {
	t.Parallel()
	m := openForm(t)

	// enter moves to the next field rather than saving.
	_, cmd := m.updateEditTenantView(keyPress("enter"))
	assert.Nil(t, cmd)
	assert.Equal(t, tenantform.FieldInternal, m.editTenant.Focus())

	// Saving without a name marks Name, focuses it, and saves nothing.
	_, cmd = m.updateEditTenantView(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Nil(t, cmd)
	assert.Equal(t, "required", m.editTenant.NameErr())
	assert.Equal(t, tenantform.FieldName, m.editTenant.Focus())
	assert.Contains(t, ansi.Strip(m.editTenantView()), "required")
	assert.Nil(t, m.toasts.Active(), "inline, not a toast")

	// Typing a name clears the mark; enter through to the last field saves.
	_, _ = m.updateEditTenantView(keyPress("a"))
	assert.Empty(t, m.editTenant.NameErr())
	_, _ = m.updateEditTenantView(keyPress("enter"))
	_, _ = m.updateEditTenantView(keyPress("enter"))
	require.Equal(t, tenantform.FieldNote, m.editTenant.Focus())
	_, cmd = m.updateEditTenantView(keyPress("enter"))
	assert.NotNil(t, cmd, "enter on the last field saves")

	// ctrl+s saves from any field.
	m.editTenant = m.editTenant.FocusOn(tenantform.FieldName)
	_, cmd = m.updateEditTenantView(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
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
	assert.True(t, m.editTenant.Visible(), "the form keeps the input")
}
