package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

const (
	helpKeyCol    = 12 // key column width inside a section
	helpColGap    = 4  // spaces between help columns
	helpMaxColumn = 3
)

// helpSection is one titled group of key bindings, rendered as lines.
type helpSection []string

// helpSections renders every key group that applies to the view help was
// opened from.
func (m *Model) helpSections() []helpSection {
	var out []helpSection
	add := func(title string, bind []key.Binding) {
		sec := helpSection{m.theme.HelpHeader.Render(title)}
		for _, b := range bind {
			h := b.Help()
			if h.Key == "" && h.Desc == "" {
				continue
			}
			sec = append(sec, "  "+m.theme.HelpKey.Render(fmt.Sprintf("%-*s", helpKeyCol, h.Key))+
				m.theme.HelpDesc.Render(h.Desc))
		}
		if len(sec) > 1 {
			out = append(out, sec)
		}
	}
	add("Quick Switch", m.quickSwitchBindings(m.lastViewMode))
	add("Resource Actions", m.keys.Context)
	add(fmt.Sprintf("%s View Actions", m.lastViewMode), m.keys.Mode)
	add("General Actions", m.keys.Global)
	switch m.lastViewMode {
	case common.ListView:
		add("Table Actions", m.tableBinding())
	case common.DetailsView:
		add("Viewport Actions", m.viewportBinding())
	case common.LoadingView, common.HelpView, common.ExportView, common.EditTenantView,
		common.LogView, common.ConfirmView, common.RowPickerView:
		// No additional sections for these view modes
	}
	return out
}

// helpLines lays the sections out in as many columns (up to three) as the
// modal's width allows and returns the resulting body lines.
func (m *Model) helpLines() []string {
	secs := m.helpSections()
	secW := 0
	for _, sec := range secs {
		for _, l := range sec {
			secW = max(secW, lipgloss.Width(l))
		}
	}
	inner := m.viewWidth*9/10 - 4 // modal border and padding
	cols := min(max((inner+helpColGap)/(secW+helpColGap), 1), helpMaxColumn)
	columns := packColumns(secs, cols)

	pad := lipgloss.NewStyle().Width(secW)
	parts := make([]string, 0, 2*len(columns))
	for i, col := range columns {
		if i > 0 {
			parts = append(parts, strings.Repeat(" ", helpColGap))
		}
		parts = append(parts, pad.Render(strings.Join(col, "\n")))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, parts...), "\n")
}

// packColumns fills up to n columns top to bottom, in order, starting a
// new column once the current one would pass an even share of the lines.
// Sections in one column are separated by a blank line.
func packColumns(secs []helpSection, n int) [][]string {
	total := 0
	for _, sec := range secs {
		total += len(sec) + 1
	}
	target := (total + n - 1) / n
	columns := [][]string{nil}
	for _, sec := range secs {
		cur := &columns[len(columns)-1]
		if len(*cur) > 0 && len(*cur)+1+len(sec) > target && len(columns) < n {
			columns = append(columns, nil)
			cur = &columns[len(columns)-1]
		}
		if len(*cur) > 0 {
			*cur = append(*cur, "")
		}
		*cur = append(*cur, sec...)
	}
	return columns
}

// helpWindow is how many body lines help shows at once, and the largest
// scroll offset; maxScroll is 0 when everything fits.
func (m *Model) helpWindow(lines int) (rows, maxScroll int) {
	body := m.modalMaxBody()
	if lines <= body {
		return lines, 0
	}
	rows = max(body-1, 1) // one line for the scroll indicator
	return rows, lines - rows
}

// fullHelpView renders the help modal: key groups in columns, scrolled
// to m.helpScroll with an indicator when they don't fit.
func (m *Model) fullHelpView() string {
	p := m.theme.Palette
	lines := m.helpLines()
	rows, maxScroll := m.helpWindow(len(lines))
	off := min(max(m.helpScroll, 0), maxScroll)
	body := strings.Join(lines[off:off+rows], "\n")
	hints := "esc close"
	if maxScroll > 0 {
		hints = "j/k pgup/pgdn gg/G scroll · esc close"
		var where string
		switch off {
		case 0:
			where = "↓ more"
		case maxScroll:
			where = "↑ more"
		default:
			where = "↑↓ more"
		}
		body += "\n" + lipgloss.NewStyle().Foreground(p.Muted).
			Render(fmt.Sprintf("%s · %d%%", where, off*100/maxScroll))
	}
	return m.modal("Keys · "+m.lastViewMode.String()+" view", body, hints, p.Focus, 0)
}

// scrollHelp moves the help body by delta lines (or to an end), clamped
// to the content.
func (m *Model) scrollHelp(k string) {
	rows, maxScroll := m.helpWindow(len(m.helpLines()))
	switch k {
	case "j", "down":
		m.helpScroll++
	case "k", "up":
		m.helpScroll--
	case "pgdown", "f", "space":
		m.helpScroll += rows
	case "pgup", "b":
		m.helpScroll -= rows
	case "home":
		m.helpScroll = 0
	case "end", "G":
		m.helpScroll = maxScroll
	}
	m.helpScroll = min(max(m.helpScroll, 0), maxScroll)
}
