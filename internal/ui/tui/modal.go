package tui

import (
	"image/color"

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
// width is the outer width; 0 fits the content.
func (m *Model) modal(title, body, hints string, border color.Color, width int) string {
	if hints != "" {
		body += "\n\n" + lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).Render(hints)
	}
	titleStyle := lipgloss.NewStyle().Foreground(border).Bold(true)
	return view.Box(title, body, border, titleStyle, width)
}

// overlayModal draws modal centered over the view it was opened from,
// which is dimmed so the modal stands out while its context stays
// visible.
func (m *Model) overlayModal(under common.ViewMode, modal string) string {
	if isModalView(under) {
		under = common.ListView
	}
	base := view.Dim(m.renderMode(under), m.theme.Palette.Muted)
	return view.Overlay(base, modal, m.viewWidth, m.viewHeight)
}

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
