/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

func (m *Model) infoView() string {
	k8sVersion := m.k8sVersion
	if k8sVersion == "" {
		k8sVersion = "…"
	}
	keys := []string{"Realm:", "Type:", "Region:", "Context:", "K8s:", "Version:"}
	values := []string{
		m.environment.Realm,
		m.environment.Type,
		m.environment.Region,
		m.environment.KubeContext(),
		k8sVersion,
		m.version,
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
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
	fmt.Fprintf(&statsText, "[%d/%d]", m.table.Cursor()+1, len(m.table.Rows()))
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
	prefixCell := m.prefixCell()
	inputWidth := max(m.viewWidth-w(contextCell)-w(prefixCell)-w(loadingCell)-w(liveCell)-w(statsCell)-
		w(m.textInput.Prompt)-1, 0)
	ti := *m.textInput
	ti.SetWidth(inputWidth)
	inputCell := ti.View()

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		contextCell,
		inputCell,
		prefixCell,
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

// View renders the current state of the model as a full-screen frame.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// render draws the active view as a string, with any toasts and the
// which-key popup composited over its bottom-right corner so they never
// shift the layout.
func (m *Model) render() string {
	body := m.renderMode(m.viewMode)
	var corner []string
	if m.toasts.active != nil {
		corner = append(corner, m.renderToasts())
	}
	if m.prefix.whichKey {
		corner = append(corner, m.whichKeyBox())
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
		spin := m.loadingSpinner.View()
		sw := m.loadingTimer.View()
		return m.centered(fmt.Sprintf("%s Loading data: %s … %s", spin, m.category.String(), sw))
	case common.HelpView:
		return m.overlayModal(m.lastViewMode, m.fullHelpView())
	case common.ListView:
		return m.frame(m.theme.Base.Render(m.table.View()))
	case common.DetailsView:
		return m.frame(m.viewport.View())
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
	default:
		return ""
	}
}

// headerInfo renders the header's info column as frame() draws it; layout
// budgets must measure this, not the unstyled infoView, whose width the
// InfoValue style pads out.
func (m *Model) headerInfo() string {
	return m.theme.InfoValue.Render(m.infoView())
}

// showLogo reports whether the logo fits beside the info column in a
// terminal w columns wide. Help yields its space first, so the logo drops
// only when info and logo alone no longer fit.
func (m *Model) showLogo(w int) bool {
	return w >= lipgloss.Width(m.headerInfo())+view.LogoWidth+view.LogoMargin
}

// centered is a thin wrapper around view.CenterText using model dims.
func (m *Model) centered(msg string) string {
	return view.CenterText(msg, m.viewWidth, m.viewHeight)
}

// frame builds the common header + status frame and injects main.
func (m *Model) frame(main string) string {
	km := m.keys
	km.QuickSwitch = m.quickSwitchBindings(m.viewMode)
	// Measure on an unbounded copy: the real bubble truncates at Width,
	// which would cap the measurement and hide overflow from HeaderHelp.
	measure := func(cols [][]key.Binding) int {
		h := *m.help
		h.SetWidth(0)
		return lipgloss.Width(h.FullHelpView(cols))
	}
	// A zero budget means no room, but bubbles and HeaderHelp both read
	// Width 0 as unbounded, so skip help rather than render it all.
	var helpView string
	if m.help.Width() > 0 {
		helpView = m.help.FullHelpView(km.HeaderHelp(m.help.Width(), measure))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, m.headerInfo(), helpView)
	if m.showLogo(m.viewWidth) {
		logo := view.RenderLogo(m.theme.LogoTop, m.theme.LogoBottom)
		header = lipgloss.JoinHorizontal(lipgloss.Top, header,
			lipgloss.PlaceHorizontal(max(m.viewWidth-lipgloss.Width(header), 0), lipgloss.Right, logo))
	}
	status := m.statusView()
	return lipgloss.JoinVertical(lipgloss.Left, header, status, main)
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
