package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestHandleSetFilterMsg(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		textInput: &ti,
	}
	msg := SetFilterMsg{Text: "foo"}
	cmd := m.handleSetFilterMsg(msg)
	assert.Equal(t, "foo", m.newFilter)
	assert.Equal(t, "foo", m.textInput.Value())
	assert.NotNil(t, cmd)
}

func TestHandleSpinnerTickMsg(t *testing.T) {
	s := spinner.New()
	m := &Model{
		loadingSpinner: &s,
	}
	msg := spinner.TickMsg{}
	cmd := m.handleSpinnerTickMsg(msg)
	assert.NotNil(t, cmd)
}

func TestHandleNextCategory(t *testing.T) {
	m := &Model{
		category: domain.Tenant,
	}
	cmd := m.handleNextCategory()
	assert.NotNil(t, cmd)
}

func TestHandlePrevCategory(t *testing.T) {
	m := &Model{
		category: domain.Tenant,
	}
	cmd := m.handlePrevCategory()
	assert.NotNil(t, cmd)
}
