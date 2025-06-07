package view

import (
	"github.com/charmbracelet/lipgloss"
)

// CenterText centers the given text within the specified width and height.
func CenterText(text string, width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(text)
}
