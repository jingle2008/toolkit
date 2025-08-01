package view

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// Renderer abstracts rendering JSON to a string (for viewport/detail).
type Renderer interface {
	RenderJSON(data any, width int) (string, error)
}

// ProductionRenderer uses glamour for markdown rendering.
type ProductionRenderer struct{}

// RenderJSON renders the given data as JSON in a markdown code block using glamour.
func (ProductionRenderer) RenderJSON(data any, width int) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(styles.DarkStyle), // auto style has delay of 5 to 10s after upgrade
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("error creating TermRenderer: %w", err)
	}
	details := fmt.Sprintf("```json\n%v\n```", data)
	return renderer.Render(details)
}
