package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvBar_ShowsLiveIndicatorWhenWatching(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewWidth = 120
	m.viewHeight = 40
	m.updateLayout(m.viewWidth, m.viewHeight)

	m.watch.k8sActive = false
	off := m.envBar()
	m.watch.k8sActive = true
	on := m.envBar()

	assert.NotEqual(t, off, on, "live indicator must change the env bar")
	assert.True(t, strings.Contains(on, "LIVE") || strings.Contains(on, "●"),
		"expected a live marker in the env bar, got %q", on)
}
