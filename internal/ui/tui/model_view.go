/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

func truncateString(s string, limit int) string { return view.Truncate(s, limit) }

// detailRenderWidth is the details body width inside its panel border.
func (m *Model) detailRenderWidth() int {
	return max(m.viewWidth-2, 0)
}

func (m *Model) updateContentAsync() tea.Cmd {
	if m.viewMode != common.DetailsView {
		return nil
	}

	gen := m.gens.nextDetail()
	// The Cmd renders a copy (snapshotItem), never the dataset's element.
	item := snapshotItem(findItem(m.dataset, m.category, m.selectedKey))
	// An item deleted while details is open stays on screen, marked gone,
	// rendered from what details last showed.
	if m.detailGone = isNilItem(item) && m.detailItem != nil; m.detailGone {
		item = m.detailItem
	}
	m.detailItem = item
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
		str, err := renderer.RenderJSON(content, max(width-2, 1))
		return detailContentRenderedMsg{Content: indentLines(str, " "), Err: err, Gen: gen} // the fields layout's margin
	}
}

// refreshStaleDetail re-renders an open details view after its rows were
// rebuilt, so it shows the item as the latest reload has it. The viewport
// keeps its scroll position.
func (m *Model) refreshStaleDetail() tea.Cmd {
	if !m.detailStale || m.viewMode != common.DetailsView {
		return nil
	}
	m.detailStale = false
	return m.updateContentAsync()
}

// detailTitle is the details panel's title: the item's key, marked gone
// once it has left the dataset.
func (m *Model) detailTitle() string {
	if m.detailGone {
		return itemKeyString(m.selectedKey) + " · gone"
	}
	return itemKeyString(m.selectedKey)
}

// isNilItem reports whether a looked-up item is missing: nil, or a nil
// pointer in an interface (what a typed lookup returns for no match).
func isNilItem(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
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
func indentLines(s, pad string) string { return view.IndentLines(s, pad) }

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
	if m.toasts.Visible() {
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
		return m.frame(m.detailTitle(), m.viewport.View())
	case common.ExportView:
		return m.overlayModal(m.lastViewMode, m.exportView())
	case common.EditTenantView:
		return m.overlayModal(m.lastViewMode, m.editTenantView())
	case common.LogView:
		return m.logView()
	case common.ConfirmView:
		return m.overlayModal(m.confirmPending.returnView, m.confirmView())
	case common.RowPickerView:
		return m.overlayModal(m.rowPickerPending.returnView, m.rowPickerView())
	case common.PaletteView:
		return m.overlayModal(m.palettePending.returnView, m.paletteView())
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
	if len(m.table.Rows()) > 0 {
		return m.markedTableView()
	}
	body := m.table.View()
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

// selectionMarker sits in the selected row's left padding, in Accent, so
// the selection reads at a glance on its quiet Selection tint.
const selectionMarker = "▌"

// rowTag is a zero-width mark set before the selected row's first cell so
// its padding can be found in the rendered table.
const rowTag = "\u200b"

// taggedPad matches the padding before rowTag, with any styling between.
var taggedPad = regexp.MustCompile(" ((?:\x1b\\[[0-9;:]*m)*)" + rowTag)

// markedTableView is the table with the selection marker in the selected
// row's left padding cell, so the row's text stays aligned with the rest.
func (m *Model) markedTableView() string {
	rows, c := m.table.Rows(), m.table.Cursor()
	if c < 0 || c >= len(rows) || len(rows[c]) == 0 {
		return m.table.View()
	}
	marker := selectionMarker
	if a := m.theme.Palette.Accent; a != nil {
		marker = fgSeq(a) + selectionMarker + "\x1b[39m"
	}
	// The table renders its visible rows ahead of View, so render a copy
	// of it whose selected row carries the tag. The copy gets its own row
	// slice and row, and SetRows replaces (never edits) the viewport's
	// lines, so the model's table is left untouched: View has no side
	// effects.
	tagged := slices.Clone(rows)
	tagged[c] = slices.Clone(rows[c])
	tagged[c][0] = rowTag + rows[c][0]
	t := *m.table
	t.SetRows(tagged)
	return taggedPad.ReplaceAllString(t.View(), marker+"$1")
}

// emptyMessage says why the list has no rows: still loading, the load
// failed, the filter or faulty-only mode hides them all, or there are
// none here.
func (m *Model) emptyMessage() string {
	cat := m.category.String()
	switch {
	case m.pendingTasks > 0:
		return "Loading " + cat + "…"
	case m.loadFailed:
		msg := "Couldn't load " + cat
		if m.canRefresh() {
			msg += " · " + keys.Refresh.Help().Key + " retry"
		}
		return msg + " · ` shows the error"
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
