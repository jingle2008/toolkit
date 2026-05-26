package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrMsg(t *testing.T) {
	t.Parallel()
	err := errors.New("fail")
	msg := errMsg(err)
	assert.Equal(t, err, msg)
}

func TestDataMsg(t *testing.T) {
	t.Parallel()
	data := 42
	msg := dataMsg{Data: data}
	assert.Equal(t, 42, msg.Data)
}

func TestFilterMsg(t *testing.T) {
	t.Parallel()
	msg := filterMsg("foo")
	assert.Equal(t, "foo", string(msg))
}

func TestSetFilterMsg(t *testing.T) {
	t.Parallel()
	msg := setFilterMsg("bar")
	assert.Equal(t, "bar", string(msg))
}
