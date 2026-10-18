package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/internal/uistate"
)

// keymapVersion is the TUI keymap's version. Raise it when keys move, and
// update keysHintLines to say what changed: users whose saved version is
// lower see the hint once.
//
//	2: sorting on s + column letter, G to the bottom, : palette, v preview.
const keymapVersion = 2

// keysHintLines is what the keys-changed hint says.
var keysHintLines = []string{
	"Sorting moved to s + the column's letter (s n, s t, …)",
	"G goes to the bottom · : opens the command palette",
	"v shows a preview pane on wide terminals",
	"? lists every key · press any key to dismiss",
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
			logger.Warnw("failed to save ui state", "path", path, "error", err)
		}
		return nil
	}
}

// keysHintBox draws the hint for the bottom-right corner.
func (m *Model) keysHintBox() string {
	p := m.theme.Palette
	return view.Box("Keys changed", strings.Join(keysHintLines, "\n"), p.Info, lipgloss.NewStyle().Foreground(p.Info).Bold(true), 0)
}
