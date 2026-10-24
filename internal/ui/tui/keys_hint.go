package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/internal/uistate"
)

// keymapVersion is the TUI keymap's version. Raise it when keys move, and
// add what changed to keysHintChanges: users whose saved version is lower
// see the hint once, listing every change since that version.
//
//	2: sorting on s + column letter, G to the bottom, : palette, v preview.
//	3: s z sorts by Size (s s is Status in every category).
const keymapVersion = 3

// keysHintChanges is what changed in each keymap version, newest first.
var keysHintChanges = []struct {
	version int
	lines   []string
}{
	{3, []string{"s z sorts by Size; s s is Status everywhere"}},
	{2, []string{
		"Sorting moved to s + the column's letter (s n, s t, …)",
		"G goes to the bottom · : opens the command palette",
		"v shows a preview pane on wide terminals",
	}},
}

// keysHintLines is what the keys-changed hint says to a user who last saw
// keymap version saved: the changes since, then how to dismiss it.
func keysHintLines(saved int) []string {
	var out []string
	for _, c := range keysHintChanges {
		if c.version > saved {
			out = append(out, c.lines...)
		}
	}
	return append(out, "? lists every key · press any key to dismiss")
}

// WithUIState sets the persisted UI state and the file it is saved to.
// The keys-changed hint shows when the state predates keymapVersion. An
// empty path disables the hint and saving.
func WithUIState(s uistate.State, path string) ModelOption {
	return func(m *Model) {
		m.uiState, m.uiStatePath = s, path
		m.keysHint = path != "" && s.KeymapVersion < keymapVersion
	}
}

// dismissKeysHint hides the hint and saves the current keymap version, so
// it doesn't show again.
func (m *Model) dismissKeysHint() tea.Cmd {
	m.keysHint = false
	m.uiState.KeymapVersion = keymapVersion
	s, path, logger := m.uiState, m.uiStatePath, m.logger
	return func() tea.Msg {
		if err := s.Save(path); err != nil && logger != nil {
			logger.Warn("failed to save ui state", "path", path, "error", err)
		}
		return nil
	}
}

// keysHintBox draws the hint for the bottom-right corner.
func (m *Model) keysHintBox() string {
	p := m.theme.Palette
	return view.Box("Keys changed", strings.Join(keysHintLines(m.uiState.KeymapVersion), "\n"), p.Accent, lipgloss.NewStyle().Foreground(p.Accent).Bold(true), 0)
}
