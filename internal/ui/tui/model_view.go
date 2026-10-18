/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"charm.land/bubbles/v2/key"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// contextString is the breadcrumb path: the category, then the scope it
// is narrowed to. The details view's row is its panel's title, so the
// path doesn't repeat it.
func (m *Model) contextString() string {
	path := m.category.String()
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
	asJSON := m.detailJSON
	styles := m.fieldStyles()

	return func() tea.Msg {
		if !asJSON {
			tree, err := itemFields(item)
			if err != nil {
				return detailContentRenderedMsg{Err: err, Gen: gen}
			}
			// A column of margin each side, as the JSON's code block has.
			body := renderFields(tree, max(width-2, 1), 0, true, true, styles)
			return detailContentRenderedMsg{Content: indentLines(body, " "), Gen: gen}
		}
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
		// The other layout renders without what failed.
		other := "tab shows JSON"
		if m.detailJSON {
			other = "tab shows fields"
		}
		return m.showErrorWithNext(fmt.Sprintf("render failed: %v", msg.Err), other)
	}
	m.viewport.SetContent(msg.Content)
	return nil
}

// indentLines prefixes every line of s with pad.
func indentLines(s, pad string) string {
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
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
		return m.frame(m.tableTitle(), m.listBody())
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

// listBody is the table, or, when it has no rows, its header over a
// message saying why it is empty and what to do about it.
func (m *Model) listBody() string {
	body := m.table.View()
	if len(m.table.Rows()) > 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	const header = 2 // column titles and their rule
	// A table too short for any row below its header has no room for the
	// message.
	if len(lines) <= header {
		return body
	}
	width := m.table.Width()
	msg := lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).
		Render(truncateString(m.emptyMessage(), max(width, 1)))
	lines[header+(len(lines)-header)/3] = lipgloss.PlaceHorizontal(width, lipgloss.Center, msg)
	return strings.Join(lines, "\n")
}

// emptyMessage says why the list has no rows: still loading, the filter
// or faulty-only mode hides them all, or there are none here.
func (m *Model) emptyMessage() string {
	cat := m.category.String()
	switch {
	case m.pendingTasks > 0:
		return "Loading " + cat + "…"
	case m.filter != "":
		return fmt.Sprintf("Nothing matches /%s · esc clears the filter", m.filter)
	case m.showFaulty:
		return fmt.Sprintf("No faulty %s · %s shows all", cat, keys.ToggleFaulty.Help().Key)
	case m.scope != nil && m.scope.Category.IsScopeOf(m.category):
		return fmt.Sprintf("No %s in %s", cat, m.scope.Name)
	case m.category.IsEnvironmentScoped():
		return fmt.Sprintf("No %s in %s", cat, m.environment.KubeContext())
	default:
		return "No " + cat
	}
}
