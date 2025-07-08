package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestHandleSetFilterMsg(t *testing.T) {
	t.Parallel()
	ti := textinput.New()
	m := &Model{
		textInput: &ti,
	}
	msg := SetFilterMsg("foo")
	cmd := m.handleSetFilterMsg(msg)
	assert.Equal(t, "foo", m.newFilter)
	assert.Equal(t, "foo", m.textInput.Value())
	assert.NotNil(t, cmd)
}

func TestHandleSpinnerTickMsg(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		loadingSpinner: &s,
	}
	msg := spinner.TickMsg{}
	cmd := m.handleSpinnerTickMsg(msg)
	assert.NotNil(t, cmd)
}

func TestHandleNextCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
	}
	cmd := m.handleNextCategory()
	assert.NotNil(t, cmd)
}

func TestHandlePrevCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{
		category:       domain.Tenant,
		loadingSpinner: &s,
	}
	cmd := m.handlePrevCategory()
	assert.NotNil(t, cmd)
}
