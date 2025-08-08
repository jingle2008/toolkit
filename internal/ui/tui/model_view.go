/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"sort"
	"strings"

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
		m.environment.GetKubeContext(),
		m.version,
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.infoKeyStyle.Render(strings.Join(keys, "\n")),
		" ",
		m.infoValueStyle.Render(strings.Join(values, "\n")),
	)

	return content
}

func (m *Model) contextString() string {
	scope := "all"
	if m.viewMode == common.DetailsView {
		scope = getItemKeyString(m.choice)
	} else if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		scope = m.context.Name
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
	contextCell := m.contextStyle.Render(ctx)

	statsText := strings.Builder{}
	if m.stats != nil {
		keys := make([]string, 0, len(m.stats))
		for k := range m.stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			statsText.WriteString(fmt.Sprintf("%s: %d • ", k, m.stats[k]))
		}
	}
	statsText.WriteString(fmt.Sprintf("[%d/%d]", m.table.Cursor()+1, len(m.table.Rows())))
	statsCell := m.statsStyle.Render(statsText.String())

	m.textInput.Width = m.viewWidth - w(contextCell) - w(statsCell) -
		w(m.textInput.Prompt) - // prompt takes extra space
		1 // 1 for cursor
	inputCell := m.textInput.View()

	return lipgloss.JoinHorizontal(lipgloss.Top,
		contextCell,
		inputCell,
		statsCell,
	)
}

func (m *Model) updateContent(width int) {
	if m.viewMode != common.DetailsView {
		return
	}

	var err error
	item := findItem(m.dataset, m.category, m.choice)
	content, err := jsonutil.PrettyJSON(item)
	if err != nil {
		content = err.Error()
	}
	str, err := m.renderer.RenderJSON(content, width)
	if err != nil {
		m.err = fmt.Errorf("error encountered rendering content: %w", err)
		return
	}
	m.viewport.SetContent(str)
}

// View renders the current state of the model as a string.
func (m *Model) View() string {
	// exhaustive:common.ViewMode
	switch m.viewMode {
	case common.LoadingView:
		spin := m.loadingSpinner.View()
		sw := m.loadingTimer.View()
		return m.centered(fmt.Sprintf("%s Loading data: %s … %s", spin, m.category.String(), sw))
	case common.ErrorView:
		return m.centered(m.err.Error())
	case common.HelpView:
		return m.centered(m.fullHelpView())
	case common.ListView:
		return m.frame(m.baseStyle.Render(m.table.View()))
	case common.DetailsView:
		return m.frame(m.viewport.View())
	case common.ExportView:
		return m.centered(m.exportView())
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
	infoView := m.infoValueStyle.Render(m.infoView())
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
			m.helpKey.Render(fmt.Sprintf("%-*s", keyCol, k)),
			m.helpDesc.Render(d))
	}
	renderSection := func(title string, bind []key.Binding) {
		if len(bind) == 0 {
			return
		}
		fmt.Fprintln(&b, m.helpHeader.Render(title))
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
		renderSection("Table Actions", m.getTableBinding())
	case common.DetailsView:
		renderSection("Viewport Actions", m.getViewportBinding())
	case common.LoadingView, common.HelpView, common.ErrorView, common.ExportView:
		// No additional sections for these view modes
	}
	return m.helpBorder.Width(m.viewWidth / 2).Render(b.String())
}

func (m *Model) getTableBinding() []key.Binding {
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

func (m *Model) getViewportBinding() []key.Binding {
	return []key.Binding{
		m.viewport.KeyMap.Up,
		m.viewport.KeyMap.Down,
		m.viewport.KeyMap.HalfPageUp,
		m.viewport.KeyMap.HalfPageDown,
		m.viewport.KeyMap.PageUp,
		m.viewport.KeyMap.PageDown,
	}
}
