package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// previewMinWidth is the narrowest terminal that shows the preview pane;
// below it the table needs every column.
const previewMinWidth = 160

// previewShown reports whether the list view draws the preview pane now:
// toggled on, in list view, on a wide enough terminal.
func (m *Model) previewShown() bool {
	return m.preview && m.viewMode == common.ListView && m.viewWidth >= previewMinWidth
}

// previewOuterWidth is the preview panel's width, borders included: 40%
// of the terminal.
func (m *Model) previewOuterWidth() int {
	return m.viewWidth * 2 / 5
}

// togglePreview flips the preview pane for the session. On a terminal too
// narrow for it, it says so instead.
func (m *Model) togglePreview() tea.Cmd {
	if !m.preview && m.viewWidth < previewMinWidth {
		return m.showToast(fmt.Sprintf("preview needs ≥%d columns (have %d)", previewMinWidth, m.viewWidth), toastInfo)
	}
	m.preview = !m.preview
	m.updateLayout(m.viewWidth, m.viewHeight)
	return nil
}

// previewPanel renders the selected row's summary in a panel titled with
// its name, height rows tall to match the table's panel.
func (m *Model) previewPanel(height int) string {
	p := m.theme.Palette
	width := m.previewOuterWidth()
	row := m.selectedRawRow()
	title := ""
	if len(row) > 0 {
		title = row[0]
	}
	body := m.previewBody(width - 4) // borders and padding
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return view.Box(title, strings.Join(lines, "\n"), p.Border,
		lipgloss.NewStyle().Foreground(p.Primary).Bold(true), width)
}

// previewBody is the selected item's fields, width columns wide: one
// level of nested objects as indented sections, deeper ones collapsed,
// each value cut to one line.
func (m *Model) previewBody(width int) string {
	muted := lipgloss.NewStyle().Foreground(m.theme.Palette.Muted)
	item := findItem(m.dataset, m.category, itemKeyFrom(m.category, m.selectedRawRow()))
	if item == nil {
		return muted.Render("nothing selected")
	}
	tree, err := itemFields(item)
	if err != nil {
		return muted.Render(err.Error())
	}
	return renderFields(tree, width, 1, false, false, m.fieldStyles())
}

// itemFields is item's JSON, as the details view and copy show it, as a
// node tree.
func itemFields(item any) (node, error) {
	content, err := jsonutil.Pretty(item)
	if err != nil {
		return node{}, err
	}
	return parseFields([]byte(content))
}

// fieldStyles colors a fields layout from the theme.
func (m *Model) fieldStyles() fieldStyles {
	p := m.theme.Palette
	return fieldStyles{
		label:   lipgloss.NewStyle().Foreground(p.Muted),
		section: lipgloss.NewStyle().Foreground(p.Primary).Bold(true),
	}
}
