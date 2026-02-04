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
