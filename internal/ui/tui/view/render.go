package view

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	interrors "github.com/jingle2008/toolkit/internal/errors"
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
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", interrors.Wrap("error creating TermRenderer", err)
	}
	details := fmt.Sprintf("```json\n%v\n```", data)
	return renderer.Render(details)
}
