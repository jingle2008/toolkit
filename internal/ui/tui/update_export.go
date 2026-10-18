package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

const (
	// exportListMax is the most folders the export modal lists at once.
	exportListMax = 12
	// exportChrome is the rows the export modal spends around its folder
	// list: the folder and name lines and the blank under them.
	exportChrome = 3
)

// exportHints is the export modal's key line, in the shared modal style.
const exportHints = "enter export · ↑/↓ select · → open · ← up · tab rename · esc cancel"

// enterExportView opens the export modal over the current view: the
// folder list re-read, the name reset to the category's default, focus on
// the list.
func (m *Model) enterExportView() []tea.Cmd {
	m.lastViewMode = m.viewMode
	m.viewMode = common.ExportView
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue(m.exportFilename())
	m.exportName = ti
	m.naming = false
	return []tea.Cmd{m.dirPicker.Init()}
}

// updateExportView handles the export modal. enter exports into the
// folder shown, under the name shown; tab moves between the folder list
// and the name; esc (and e, from the list) cancels.
func (m *Model) updateExportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportDoneMsg:
		m.viewMode = m.lastViewMode
		return m, m.showToast("exported to "+tildePath(msg.path), toastInfo)
	case exportErrMsg:
		m.viewMode = m.lastViewMode
		return m, m.showToast(fmt.Sprintf("export failed: %v", msg.err), toastError)
	}

	if km, ok := msg.(tea.KeyPressMsg); ok {
		if cmd, handled := m.exportKey(km); handled {
			return m, cmd
		}
	}

	var cmd tea.Cmd
	if m.naming {
		m.exportName, cmd = m.exportName.Update(msg)
		return m, cmd
	}
	updated, cmd := m.dirPicker.Update(msg)
	m.dirPicker = &updated
	return m, cmd
}

// exportKey runs the modal's own keys — quit, cancel, focus, export —
// and reports whether km was one. While the name has focus, q and e are
// text, not keys.
func (m *Model) exportKey(km tea.KeyPressMsg) (tea.Cmd, bool) {
	switch k := km.String(); {
	case k == "ctrl+c" || (!m.naming && key.Matches(km, keys.Quit)):
		m.cancelInFlight()
		return tea.Quit, true
	case k == "esc" || (!m.naming && key.Matches(km, keys.ExportCSV)):
		m.viewMode = m.lastViewMode
		return nil, true
	case k == "tab" || k == "shift+tab":
		m.naming = !m.naming
		if m.naming {
			return m.exportName.Focus(), true
		}
		m.exportName.Blur()
		return nil, true
	case k == "enter":
		return m.startExport(), true
	}
	return nil, false
}

// startExport writes the CSV to the folder shown under the name given,
// or toasts why the name won't do.
func (m *Model) startExport() tea.Cmd {
	name := strings.TrimSpace(m.exportName.Value())
	switch {
	case name == "":
		return m.showToast("file name is required", toastWarn)
	case strings.ContainsRune(name, filepath.Separator):
		return m.showToast("file name can't contain "+string(filepath.Separator), toastWarn)
	}
	return m.exportTableCSVCmd(filepath.Join(m.dirPicker.CurrentDirectory, name))
}

func (m *Model) exportTableCSVCmd(outPath string) tea.Cmd {
	return func() tea.Msg {
		if err := m.exportTableCSV(outPath); err != nil {
			return exportErrMsg{err: err}
		}
		return exportDoneMsg{path: outPath}
	}
}

func (m *Model) exportFilename() string {
	return fmt.Sprintf("%s-%s.csv", m.environment.Region, m.category.String())
}

// exportView renders the export modal: the folder the file goes to, its
// name (editable), and the folders below to move into.
func (m *Model) exportView() string {
	p := m.theme.Palette
	label := lipgloss.NewStyle().Foreground(p.Muted).Width(8)
	strong := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	name := strong.Render(m.exportName.Value())
	if m.naming {
		name = m.exportName.View()
	}
	width := m.modalMaxWidth()
	field := max(width-4-label.GetWidth(), 1) // border, padding and label
	m.exportName.SetWidth(field)
	list := strings.TrimRight(m.dirPicker.View(), "\n")
	body := label.Render("Folder") + strong.Render(truncateLeft(tildePath(m.dirPicker.CurrentDirectory), field)) + "\n" +
		label.Render("Name") + name + "\n\n" + list
	return m.modal("Export CSV", body, exportHints, p.Focus, width)
}

// truncateLeft keeps s's last limit cells, marking a cut with a leading
// "…": the end of a path is the part that tells folders apart.
func truncateLeft(s string, limit int) string {
	if runewidth.StringWidth(s) <= limit {
		return s
	}
	runes := []rune(s)
	w := 1 // the ellipsis
	i := len(runes)
	for i > 0 && w+runewidth.RuneWidth(runes[i-1]) <= limit {
		i--
		w += runewidth.RuneWidth(runes[i])
	}
	return "…" + string(runes[i:])
}

// tildePath shortens a path under the home directory to ~/….
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}
