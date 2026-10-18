package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/uistate"
)

// newHintModel is the golden DAC list with UI state saved to a temp file.
func newHintModel(t *testing.T, saved int) (*Model, string) {
	t.Helper()
	m := newGoldenModel(t, 120, 30)
	showCategory(t, m, domain.DedicatedAICluster)
	path := filepath.Join(t.TempDir(), "ui.yaml")
	WithUIState(uistate.State{KeymapVersion: saved}, path)(m)
	return m, path
}

// An older saved keymap shows the hint; the first key dismisses it, saves
// the version, and still does its job.
func TestKeysHint_ShowsOnceAndSaves(t *testing.T) {
	t.Parallel()
	m, path := newHintModel(t, 0)
	require.True(t, m.keysHint)
	assert.Contains(t, ansi.Strip(m.render()), "Keys changed")

	_, cmd := m.Update(keyPress("j"))
	assert.False(t, m.keysHint)
	assert.Equal(t, 1, m.table.Cursor(), "the key still moves the cursor")
	assert.NotContains(t, ansi.Strip(m.render()), "Keys changed")

	require.NotNil(t, cmd)
	runCmd(cmd)
	s, err := uistate.Load(path)
	require.NoError(t, err)
	assert.Equal(t, keymapVersion, s.KeymapVersion)
}

func TestKeysHint_HiddenWhenCurrent(t *testing.T) {
	t.Parallel()
	m, _ := newHintModel(t, keymapVersion)
	assert.False(t, m.keysHint)
	assert.NotContains(t, ansi.Strip(m.render()), "Keys changed")
}

// No path (tests, embedded uses) means no hint and nothing written.
func TestKeysHint_NoPathNoHint(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 30)
	WithUIState(uistate.State{}, "")(m)
	assert.False(t, m.keysHint)
}

// runCmd runs cmd and, for a batch, each command in it, giving each a
// moment to finish: ticks and other long waits are abandoned.
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				runCmd(c)
			}
		}
	case <-time.After(time.Second):
	}
}

// The hint lists every change since the version the user last saw, and
// only those.
func TestKeysHint_ListsChangesSinceSaved(t *testing.T) {
	t.Parallel()
	m, _ := newHintModel(t, 2)
	out := ansi.Strip(m.render())
	assert.Contains(t, out, "s z sorts by Size")
	assert.NotContains(t, out, "Sorting moved to s")

	assert.Contains(t, keysHintLines(0), "Sorting moved to s + the column's letter (s n, s t, …)")
	assert.Contains(t, keysHintLines(0), "s z sorts by Size; s s is Status everywhere")
}
