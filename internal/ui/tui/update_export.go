package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

// updateExportView handles key events and state for the ExportView popup.
func (m *Model) updateExportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportDoneMsg:
		m.viewMode = common.ListView
		return m, nil
	case exportErrMsg:
		err := fmt.Errorf("export failed: %w", msg.err)
		m.viewMode = m.lastViewMode
		return m, m.showToast(err.Error(), toastError)
	}

	keyMsg, isKeyMsg := msg.(tea.KeyPressMsg)
	if isKeyMsg {
		switch {
		case key.Matches(keyMsg, keys.ExportCSV), key.Matches(keyMsg, keys.Back):
			m.viewMode = m.lastViewMode
			return m, nil
		case key.Matches(keyMsg, keys.Quit):
			m.cancelInFlight()
			return m, tea.Quit
		}
	}

	updated, cmd := m.dirPicker.Update(msg)
	m.dirPicker = &updated

	selected, selectedPath := m.dirPicker.DidSelectFile(msg)
	if selected && isKeyMsg && key.Matches(keyMsg, keys.Confirm) {
		exportPath := filepath.Join(selectedPath, m.exportFilename())
		return m, m.exportTableCSVCmd(exportPath)
	}

	return m, cmd
}

func (m *Model) exportTableCSVCmd(outPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.exportTableCSV(outPath); err != nil {
			return exportErrMsg{err: err}
		}
		return exportDoneMsg{}
	}
}

func (m *Model) exportFilename() string {
	return fmt.Sprintf("%s-%s.csv", m.environment.Region, m.category.String())
}

func (m *Model) exportView() string {
	p := m.theme.Palette
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Foreground(p.Muted).Render("File  "))
	s.WriteString(lipgloss.NewStyle().Foreground(p.Primary).Bold(true).Render(m.exportFilename()))
	s.WriteString("\n\n")
	s.WriteString(lipgloss.NewStyle().Foreground(p.Muted).Render("Pick a directory:"))
	s.WriteString("\n\n")
	s.WriteString(m.dirPicker.View())
	return m.modal("Export CSV", s.String(), exportHints, p.Focus, m.modalMaxWidth())
}

// exportHints is the export modal's key line, in the shared modal style.
// enter both opens a directory and picks it (the filepicker's Open and
// Select), so it is named by what the user wants from it here.
const exportHints = "↑/↓ select · → open · ← parent · enter export here · esc cancel"
