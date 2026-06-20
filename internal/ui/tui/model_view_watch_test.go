package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusView_ShowsLiveIndicatorWhenWatching(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewWidth = 120
	m.viewHeight = 40
	m.updateLayout(m.viewWidth, m.viewHeight)

	m.watching = false
	off := m.statusView()
	m.watching = true
	on := m.statusView()

	assert.NotEqual(t, off, on, "live indicator must change the status bar")
	assert.True(t, strings.Contains(on, "LIVE") || strings.Contains(on, "●"),
		"expected a live marker in the status bar, got %q", on)
}
