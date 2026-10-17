package tui

import (
	"testing"

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
	out := openForm(t).editTenantView()
	require.NotEmpty(t, out)
	assert.Contains(t, out, "abc")   // tenantID
	assert.Contains(t, out, "Name:") // field label
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

	// Empty name → a warning command, form stays open.
	_, cmd := m.handleEditTenantKey(keyPress("enter"))
	assert.NotNil(t, cmd)
	assert.NotNil(t, m.editTenant)

	// With a name → a save command.
	m.editTenant.name.SetValue("acme")
	_, cmd = m.handleEditTenantKey(keyPress("enter"))
	assert.NotNil(t, cmd)
}

func TestModel_MetadataPath(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, makeTestModel().metadataPath())
}
