// Package toolkit implements the view and rendering logic for the Model.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/internal/utils"
)

func (m *Model) infoView() string {
	keys := []string{"Realm:", "Type:", "Region:"}
	values := []string{m.environment.Realm, m.environment.Type, m.environment.Region}

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

	if m.chosen {
		keyString := getItemKeyString(m.category, m.choice)
		scope = fmt.Sprintf("%s/%s", scope, keyString)
	}

	return fmt.Sprintf("%s (%s)", m.category.String(), scope)
}

func (m *Model) statusView() string {
	w := lipgloss.Width

	contextCell := m.contextStyle.Render(m.contextString())

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
	if !m.chosen {
		return
	}

	var err error
	item := findItem(m.dataset, m.category, m.choice)
	content, err := utils.PrettyJSON(item)
	if err != nil {
		content = err.Error()
	}
	str, err := m.renderer.RenderJSON(content, width)
	if err != nil {
		wrappedErr := fmt.Errorf("error encountered rendering content: %w", err)
		m.err = wrappedErr
		return
	}
	m.viewport.SetContent(str)
}

// View renders the current state of the model as a string.
func (m *Model) View() string {
	if m.err != nil {
		return view.CenterText(m.err.Error(), m.viewWidth, m.viewHeight)
	}

	helpView := m.help.View(m.keys)
	infoView := m.infoValueStyle.
		Width(m.viewWidth - lipgloss.Width(helpView)).Render(m.infoView())
	header := lipgloss.JoinHorizontal(lipgloss.Top, infoView, helpView)

	var mainContent string
	if !m.chosen {
		mainContent = m.baseStyle.Render(m.table.View())
	} else {
		mainContent = m.viewport.View()
	}

	status := m.statusView()

	return lipgloss.JoinVertical(lipgloss.Left, header, status, mainContent)
}
