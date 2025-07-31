package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateExportView handles key events and state for the ExportView popup.
func (m *Model) updateExportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKeyMsg := msg.(tea.KeyMsg)
	if isKeyMsg {
		switch {
		case key.Matches(keyMsg, keys.ExportCSV):
			m.viewMode = m.lastViewMode
			return m, nil
		case key.Matches(keyMsg, keys.Quit):
			return m, tea.Quit
		}
	}

	updated, cmd := m.dirPicker.Update(msg)
	m.dirPicker = &updated

	selected, selectedPath := m.dirPicker.DidSelectFile(msg)
	if selected && isKeyMsg && key.Matches(keyMsg, keys.Confirm) {
		exportPath := filepath.Join(selectedPath, m.exportFilename())
		if err := m.exportTableCSV(exportPath); err != nil {
			m.err = fmt.Errorf("export failed: %w", err)
			m.viewMode = common.ErrorView
			return m, nil
		}
		m.viewMode = common.ListView
		return m, nil
	}

	return m, cmd
}

func (m *Model) exportFilename() string {
	return fmt.Sprintf("%s-%s.csv", m.environment.Region, m.category.String())
}

func (m *Model) exportView() string {
	var s strings.Builder
	s.WriteString("Export file name: ")
	s.WriteString(m.dirPicker.Styles.Selected.Render(m.exportFilename()))
	s.WriteString("\n\nPick an export path:\n\n")
	s.WriteString(m.dirPicker.View() + "\n")
	s.WriteString(m.exportHelpView())
	return m.helpBorder.Width(m.viewWidth * 4 / 5).Render(s.String())
}

func (m *Model) exportHelpView() string {
	km := m.dirPicker.KeyMap
	bindings := []key.Binding{
		km.Up, km.Down, km.GoToTop, km.GoToLast,
		km.PageUp, km.PageDown, km.Select, km.Open, km.Back,
	}
	return m.help.ShortHelpView(bindings)
}
