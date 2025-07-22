package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestUpdateErrorView_Quit(t *testing.T) {
	t.Parallel()
	m := &Model{}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	model, cmd := m.updateErrorView(msg)
	assert.Equal(t, m, model)
	assert.Nil(t, cmd)

	// Now use a key that matches keys.Quit
	quitMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd = m.updateErrorView(quitMsg)
	assert.NotNil(t, cmd)
}
