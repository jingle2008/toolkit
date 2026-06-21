package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
)

func TestFormatLogEntry(t *testing.T) {
	t.Parallel()
	e := logging.Entry{
		Time:    time.Date(2026, 6, 20, 15, 4, 5, 0, time.UTC),
		Level:   logging.LevelInfo,
		Message: "mutation begin",
		Fields:  []any{"action", "drain"},
	}
	assert.Equal(t, "15:04:05 INFO  mutation begin action=drain", formatLogEntry(e))
}

func TestRenderLogEntries_Empty(t *testing.T) {
	t.Parallel()
	m := &Model{logStore: logging.NewRingSink(4)}
	assert.Contains(t, m.renderLogEntries(80), "no log entries")
}

func TestRenderLogEntries_NilStore(t *testing.T) {
	t.Parallel()
	m := &Model{}
	assert.NotPanics(t, func() { _ = m.renderLogEntries(80) })
}

func TestRenderLogEntries_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	ring.Infow("first")
	ring.Errorw("second")
	m := &Model{logStore: ring}
	out := m.renderLogEntries(120)
	assert.Less(t, strings.Index(out, "first"), strings.Index(out, "second"))
	assert.Contains(t, out, "ERROR")
}
