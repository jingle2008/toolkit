/*
Package tui implements the view and rendering logic for the Model.
*/
package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
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
	if m.context != nil && m.context.Category.IsScopeOf(m.category) {
		scope = m.context.Name
	}

	if m.viewMode == common.DetailsView {
		keyString := getItemKeyString(m.category, m.choice)
		scope = fmt.Sprintf("%s/%s", scope, keyString)
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

	statsCell := m.statsStyle.Render(
		fmt.Sprintf("[%d/%d]", m.table.Cursor()+1, len(m.table.Rows())))
	inputCell := m.statusText.
		Width(m.viewWidth - w(contextCell) - w(statsCell)).
		Render(m.textInput.View())

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
	if m.pendingTasks > 0 {
		// Compose spinner and message
		spin := m.loadingSpinner.View()
		msg := fmt.Sprintf("%s Loading data: %s …", spin, m.category.String())
		return view.CenterText(msg, m.viewWidth, m.viewHeight)
	}
	if m.err != nil {
		return view.CenterText(m.err.Error(), m.viewWidth, m.viewHeight)
	}

	// Show full help view if in HelpView mode
	if m.viewMode == common.HelpView {
		return view.CenterText(m.fullHelpView(), m.viewWidth, m.viewHeight)
	}

	helpView := m.help.View(m.keys)
	infoView := m.infoValueStyle.
		Width(m.viewWidth - lipgloss.Width(helpView)).Render(m.infoView())
	header := lipgloss.JoinHorizontal(lipgloss.Top, infoView, helpView)

	var mainContent string
	if m.viewMode == common.ListView {
		mainContent = m.baseStyle.Render(m.table.View())
	} else {
		mainContent = m.viewport.View()
	}

	status := m.statusView()

	return lipgloss.JoinVertical(lipgloss.Left, header, status, mainContent)
}

/*
fullHelpView renders all key bindings in sections for the help screen,
with improved formatting and styling.
*/
func (m *Model) fullHelpView() string {
	km := keys.FullKeyMap()
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
	renderSection("Category-specific Actions", km.Context)
	renderSection("Mode-specific Actions", km.Mode)
	renderSection("Global Actions", km.Global)
	renderSection("Table Actions", m.getTableBinding())
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
