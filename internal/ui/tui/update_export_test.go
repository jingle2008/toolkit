package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func TestUpdateExportView_ExitAndQuit(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.lastViewMode = common.ListView

	model, cmd := m.updateExportView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys.ExportCSV.Keys()[0])})
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	assert.Nil(t, cmd)

	model, cmd = m.updateExportView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys.Quit.Keys()[0])})
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}

// Quitting from the export popup must cancel in-flight loads, like every
// other view's Quit handler — otherwise a load running when the user quits
// from the popup is left uncancelled during teardown.
func TestUpdateExportView_QuitCancelsInFlight(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.lastViewMode = common.ListView

	canceled := false
	m.loadCancel = func() { canceled = true }

	_, cmd := m.updateExportView(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys.Quit.Keys()[0])})
	if !canceled {
		t.Fatal("quit from export view did not cancel in-flight tasks")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}

func TestUpdateExportView_EscDismisses(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.lastViewMode = common.ListView

	model, cmd := m.updateExportView(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	assert.Nil(t, cmd)
}

// TestUpdateExportView_DismissRestoresLastViewMode guards against a
// regression where esc or e is changed from "restore lastViewMode" to
// a hardcoded ListView. The popup must return the user to whichever
// view they opened it from, including DetailsView.
func TestUpdateExportView_DismissRestoresLastViewMode(t *testing.T) {
	t.Parallel()
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune(keys.ExportCSV.Keys()[0])},
	} {
		m := newTestModel(t)
		m.viewMode = common.ExportView
		m.lastViewMode = common.DetailsView

		model, cmd := m.updateExportView(k)
		assert.Equal(t, common.DetailsView, model.(*Model).viewMode, "key=%v", k)
		assert.Nil(t, cmd, "key=%v", k)
	}
}
