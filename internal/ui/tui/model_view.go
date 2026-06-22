/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

func (m *Model) infoView() string {
	keys := []string{"Realm:", "Type:", "Region:", "Context:", "Version:"}
	values := []string{
		m.environment.Realm,
		m.environment.Type,
		m.environment.Region,
		m.environment.KubeContext(),
		m.version,
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.theme.InfoKey.Render(strings.Join(keys, "\n")),
		" ",
		m.theme.InfoValue.Render(strings.Join(values, "\n")),
	)

	return content
}

func (m *Model) contextString() string {
	scope := "all"
	if m.viewMode == common.DetailsView {
		scope = itemKeyString(m.selectedKey)
	} else if m.scope != nil && m.scope.Category.IsScopeOf(m.category) {
		scope = m.scope.Name
	}

	return fmt.Sprintf("%s (%s)", m.category.String(), scope)
}

func truncateString(s string, limit int) string {
	if runewidth.StringWidth(s) <= limit {
		return s
	}
	return runewidth.Truncate(s, limit-1, "…")
}

func (m *Model) statusView() string {
	w := lipgloss.Width

	maxCtx := m.viewWidth / 3
	ctx := truncateString(m.contextString(), maxCtx)
	contextCell := m.theme.Context.Render(ctx)

	statsText := strings.Builder{}
	if m.stats != nil {
		keys := make([]string, 0, len(m.stats))
		for k := range m.stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %d", k, m.stats[k]))
		}
		if len(parts) > 0 {
			statsText.WriteString(strings.Join(parts, " • "))
			statsText.WriteString(" ")
		}
	}
	statsText.WriteString(fmt.Sprintf("[%d/%d]", m.table.Cursor()+1, len(m.table.Rows())))
	statsCell := m.theme.Stats.Render(statsText.String())

	loadingCell := ""
	if m.pendingTasks > 0 {
		loadingCell = fmt.Sprintf(" %s %s ", m.loadingSpinner.View(), m.loadingTimer.View())
	}

	liveCell := ""
	// k8s categories: live while their watch is established (m.watch.k8sActive).
	// repo categories: live while the always-on working-tree watch runs.
	if m.watch.k8sActive || (m.watch.repoActive && !m.category.NeedsKubeConfig()) {
		liveCell = m.theme.Live.Render("● LIVE")
	}

	// Render-time width depends on the surrounding cells, so compute
	// it here rather than in updateLayout. We deliberately operate on
	// a copy of the textinput so View() stays pure — the original
	// *m.textInput owned by the reducer is never mutated.
	inputWidth := max(m.viewWidth-w(contextCell)-w(loadingCell)-w(liveCell)-w(statsCell)-
		w(m.textInput.Prompt)-1, 0)
	ti := *m.textInput
	ti.Width = inputWidth
	inputCell := ti.View()

	return lipgloss.JoinHorizontal(lipgloss.Top,
		contextCell,
		inputCell,
		loadingCell,
		liveCell,
		statsCell,
	)
}

func (m *Model) detailRenderWidth() int {
	border := m.viewport.Style.GetBorderStyle()
	width := m.viewWidth - (border.GetLeftSize() + border.GetRightSize())
	if width < 0 {
		return 0
	}
	return width
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

// View renders the current state of the model as a string.
//
// When a toast is up we append it under the active view's body rather
// than budget a row for it in updateLayout. The terminal scrolls one
// row to accommodate, briefly clipping the table's bottom border —
// acceptable for an 8s transient banner, and far simpler than running
// layout twice per toast-set/dismiss.
func (m *Model) View() string {
	body := m.renderActiveView()
	if m.toasts.active == nil {
		return body
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, m.renderToast(m.viewWidth))
}

func (m *Model) renderActiveView() string {
	// exhaustive:common.ViewMode
	switch m.viewMode {
	case common.LoadingView:
		spin := m.loadingSpinner.View()
		sw := m.loadingTimer.View()
		return m.centered(fmt.Sprintf("%s Loading data: %s … %s", spin, m.category.String(), sw))
	case common.HelpView:
		return m.centered(m.fullHelpView())
	case common.ListView:
		return m.frame(m.theme.Base.Render(m.table.View()))
	case common.DetailsView:
		return m.frame(m.viewport.View())
	case common.ExportView:
		return m.centered(m.exportView())
	case common.EditTenantView:
		return m.centered(m.editTenantView())
	case common.LogView:
		return m.logView()
	case common.ConfirmView:
		return m.centered(m.confirmView())
	default:
		return ""
	}
}

// centered is a thin wrapper around view.CenterText using model dims.
func (m *Model) centered(msg string) string {
	return view.CenterText(msg, m.viewWidth, m.viewHeight)
}

// frame builds the common header + status frame and injects main.
func (m *Model) frame(main string) string {
	helpView := m.help.View(m.keys)
	infoView := m.theme.InfoValue.Render(m.infoView())
	header := lipgloss.JoinHorizontal(lipgloss.Top, infoView, helpView)
	status := m.statusView()
	return lipgloss.JoinVertical(lipgloss.Left, header, status, main)
}

/*
fullHelpView renders all key bindings in sections for the help screen,
with improved formatting and styling.
*/
func (m *Model) fullHelpView() string {
	const keyCol = 12
	var b strings.Builder

	renderRow := func(k, d string) {
		fmt.Fprintf(&b, "  %s%s\n",
			m.theme.HelpKey.Render(fmt.Sprintf("%-*s", keyCol, k)),
			m.theme.HelpDesc.Render(d))
	}
	renderSection := func(title string, bind []key.Binding) {
		if len(bind) == 0 {
			return
		}
		fmt.Fprintln(&b, m.theme.HelpHeader.Render(title))
		for _, bb := range bind {
			h := bb.Help()
			if h.Key == "" && h.Desc == "" {
				continue
			}
			renderRow(h.Key, h.Desc)
		}
		b.WriteString("\n")
	}
	renderSection("Resource Actions", m.keys.Context)
	renderSection(fmt.Sprintf("%s View Actions", m.lastViewMode), m.keys.Mode)
	renderSection("General Actions", m.keys.Global)
	switch m.lastViewMode {
	case common.ListView:
		renderSection("Table Actions", m.tableBinding())
	case common.DetailsView:
		renderSection("Viewport Actions", m.viewportBinding())
	case common.LoadingView, common.HelpView, common.ExportView, common.EditTenantView:
		// No additional sections for these view modes
	}
	return m.theme.HelpBorder.Width(m.viewWidth / 2).Render(b.String())
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
