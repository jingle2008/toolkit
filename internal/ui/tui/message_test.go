package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrMsg(t *testing.T) {
	err := errors.New("fail")
	msg := ErrMsg{Err: err}
	assert.Equal(t, err, msg.Err)
}

func TestDataMsg(t *testing.T) {
	data := 42
	msg := DataMsg{Data: data}
	assert.Equal(t, 42, msg.Data)
}

func TestFilterMsg(t *testing.T) {
	msg := FilterMsg{Text: "foo"}
	assert.Equal(t, "foo", msg.Text)
}

func TestSetFilterMsg(t *testing.T) {
	msg := SetFilterMsg{Text: "bar"}
	assert.Equal(t, "bar", msg.Text)
}
