/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"charm.land/bubbles/v2/key"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// contextString is the breadcrumb path: the category, then what it is
// narrowed to — the selected row in details view, else the scope.
func (m *Model) contextString() string {
	path := m.category.String()
	if m.viewMode == common.DetailsView {
		return path + " › " + itemKeyString(m.selectedKey)
	}
	if m.scope != nil && m.scope.Category.IsScopeOf(m.category) {
		return path + " › " + m.scope.Name
	}
	return path
}

func truncateString(s string, limit int) string {
	if runewidth.StringWidth(s) <= limit {
		return s
	}
	return runewidth.Truncate(s, limit-1, "…")
}

// detailRenderWidth is the details body width inside its panel border.
func (m *Model) detailRenderWidth() int {
	return max(m.viewWidth-2, 0)
}

func (m *Model) updateContentAsync() tea.Cmd {
	if m.viewMode != common.DetailsView {
		return nil
	}

	gen := m.gens.nextDetail()
	item := findItem(m.dataset, m.category, m.selectedKey)
	width := m.detailRenderWidth()
	renderer := m.renderer

	return func() tea.Msg {
		content, err := jsonutil.Pretty(item)
		if err != nil {
			content = err.Error()
		}
		str, err := renderer.RenderJSON(content, width)
		return detailContentRenderedMsg{Content: str, Err: err, Gen: gen}
	}
}

func (m *Model) handleDetailContentRenderedMsg(msg detailContentRenderedMsg) tea.Cmd {
	if msg.Gen != m.gens.detail || m.viewMode != common.DetailsView {
		return nil
	}
	if msg.Err != nil {
		return m.showToast(fmt.Sprintf("render failed: %v", msg.Err), toastError)
	}
	m.viewport.SetContent(msg.Content)
	return nil
}

// View renders the current state of the model as a full-screen frame.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// render draws the active view as a string, with the keys-changed hint
// and any toasts composited over its bottom-right corner so they never
// shift the layout.
func (m *Model) render() string {
	body := m.renderMode(m.viewMode)
	var corner []string
	if m.keysHint {
		corner = append(corner, m.keysHintBox())
	}
	if m.toasts.active != nil {
		corner = append(corner, m.renderToasts())
	}
	if len(corner) == 0 {
		return body
	}
	return view.Corner(body, lipgloss.JoinVertical(lipgloss.Right, corner...), m.viewWidth, m.viewHeight)
}

// renderMode draws one view mode. Modal modes draw over the view they
// were opened from.
func (m *Model) renderMode(mode common.ViewMode) string {
	// exhaustive:common.ViewMode
	switch mode {
	case common.LoadingView:
		return m.loadingView()
	case common.HelpView:
		return m.overlayModal(m.lastViewMode, m.fullHelpView())
	case common.ListView:
		return m.frame(m.tableTitle(), m.table.View())
	case common.DetailsView:
		return m.frame(itemKeyString(m.selectedKey), m.viewport.View())
	case common.ExportView:
		return m.overlayModal(m.lastViewMode, m.exportView())
	case common.EditTenantView:
		return m.overlayModal(m.lastViewMode, m.editTenantView())
	case common.LogView:
		return m.logView()
	case common.ConfirmView:
		return m.overlayModal(m.confirm.returnView, m.confirmView())
	case common.RowPickerView:
		return m.overlayModal(m.rowPicker.returnView, m.rowPickerView())
	case common.PaletteView:
		return m.overlayModal(m.palette.returnView, m.paletteView())
	default:
		return ""
	}
}

func (m *Model) tableBinding() []key.Binding {
	return []key.Binding{
		m.table.KeyMap.LineUp,
		m.table.KeyMap.LineDown,
		m.table.KeyMap.HalfPageUp,
		m.table.KeyMap.HalfPageDown,
		m.table.KeyMap.PageUp,
		m.table.KeyMap.PageDown,
		m.table.KeyMap.GotoTop,
		m.table.KeyMap.GotoBottom,
	}
}

func (m *Model) viewportBinding() []key.Binding {
	return []key.Binding{
		m.viewport.KeyMap.Up,
		m.viewport.KeyMap.Down,
		m.viewport.KeyMap.HalfPageUp,
		m.viewport.KeyMap.HalfPageDown,
		m.viewport.KeyMap.PageUp,
		m.viewport.KeyMap.PageDown,
	}
}
