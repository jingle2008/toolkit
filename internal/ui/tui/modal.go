package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// modalChrome is the rows a modal spends outside its body: the top and
// bottom border plus a blank line and the key-hint line.
const modalChrome = 4

// modalMaxWidth is the widest a modal box may be: 80 columns, or 90% of a
// narrower terminal.
func (m *Model) modalMaxWidth() int {
	return min(80, m.viewWidth*9/10)
}

// modalMaxBody is how many body rows fit in a modal: 90% of the terminal
// height less the chrome. Modals with more content scroll.
func (m *Model) modalMaxBody() int {
	return max(m.viewHeight*9/10-modalChrome, 1)
}

// modal frames body in the shared modal chrome: a rounded border of color
// border with title in its top edge and hints, muted, on the last line.
// The hints stay muted throughout, keys included, unlike the footer's
// Primary keys: the footer is the screen's key guide, while a modal's
// hints sit under its body and should recede behind it.
// width is the outer width; 0 fits the content.
func (m *Model) modal(title, body, hints string, border color.Color, width int) string {
	if hints != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).Render(hints)
	}
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	return view.Box(title, body, border, titleStyle, width)
}

// fixedModal is modal for a body that a filter narrows as it is typed:
// body is padded to bodyH lines and bodyW columns, the size the unfiltered
// body takes, so the box keeps its size while filtering — and, centered,
// its place: the line being typed into doesn't move under the cursor.
func (m *Model) fixedModal(title, body, hints string, border color.Color, width, bodyW, bodyH int) string {
	if n := lipgloss.Height(body); n < bodyH {
		body += strings.Repeat("\n", bodyH-n)
	}
	if lipgloss.Width(body) < bodyW {
		body = lipgloss.NewStyle().Width(bodyW).Render(body)
	}
	return m.modal(title, body, hints, border, width)
}

// overlayModal draws modal centered over the view it was opened from,
// which is dimmed so the modal stands out while its context stays
// visible. A modal within modalFullMargin rows of the screen's height
// draws on a blank screen instead: the view would only show around it
// in fragments ("GPUN", "1/2]") that read as broken.
func (m *Model) overlayModal(under common.ViewMode, modal string) string {
	if isModalView(under) {
		under = common.ListView
	}
	// The compositor sizes the screen to the base, so a blank one still
	// spans it.
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", m.viewWidth)+"\n", m.viewHeight), "\n")
	if lipgloss.Height(modal) < m.viewHeight-modalFullMargin {
		base = view.Dim(withoutFooter(under, m.renderMode(under)), m.theme.Palette.Muted)
	}
	return view.Overlay(base, modal, m.viewWidth, m.viewHeight)
}

// withoutFooter blanks the key footer, the last line, of a framed view
// (list, details, log) drawn under a modal: its keys don't work while the
// modal is open, and the modal carries its own hint line.
func withoutFooter(mode common.ViewMode, s string) string {
	switch mode { //nolint:exhaustive // only these are framed
	case common.ListView, common.DetailsView, common.LogView:
	default:
		return s
	}
	i := strings.LastIndexByte(s, '\n')
	if i < 0 {
		return s
	}
	return s[:i+1]
}

// modalFullMargin is how many rows a modal must leave free for the view
// it was opened from to show around it.
const modalFullMargin = 4

// isModalView reports whether mode draws over another view rather than
// filling the screen itself.
func isModalView(mode common.ViewMode) bool {
	switch mode { //nolint:exhaustive // the rest are full-screen
	case common.HelpView, common.ConfirmView, common.RowPickerView,
		common.ExportView, common.EditTenantView, common.PaletteView:
		return true
	}
	return false
}
