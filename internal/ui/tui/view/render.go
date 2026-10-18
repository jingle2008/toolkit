package view

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

// Renderer abstracts rendering JSON to a string (for viewport/detail).
type Renderer interface {
	RenderJSON(data any, width int) (string, error)
}

// ProductionRenderer uses glamour for markdown rendering, in its light
// style on a light terminal background.
type ProductionRenderer struct {
	Light bool
}

// RenderJSON renders the given data as JSON in a markdown code block
// using glamour, without the blank lines and indent glamour sets around
// a document, so it starts where the details view's fields layout does.
func (r ProductionRenderer) RenderJSON(data any, width int) (string, error) {
	style := styles.DarkStyle // auto style has delay of 5 to 10s after upgrade
	if r.Light {
		style = styles.LightStyle
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("error creating TermRenderer: %w", err)
	}
	out, err := renderer.Render(fmt.Sprintf("```json\n%v\n```", data))
	if err != nil {
		return "", err
	}
	return trimMargins(out), nil
}

// trimMargins drops the blank lines around s and the indent every line
// of it shares, keeping each line's styling.
func trimMargins(s string) string {
	lines := strings.Split(s, "\n")
	blank := func(l string) bool { return strings.TrimSpace(ansi.Strip(l)) == "" }
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	indent := -1
	for _, l := range lines {
		if blank(l) {
			continue
		}
		plain := ansi.Strip(l)
		n := len(plain) - len(strings.TrimLeft(plain, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if indent > 0 {
			l = ansi.TruncateLeft(l, indent, "")
		}
		// Trailing padding can sit inside a styled run, past what a plain
		// TrimRight sees: cut at the text's own width instead.
		cut := ansi.Truncate(l, ansi.StringWidth(strings.TrimRight(ansi.Strip(l), " ")), "")
		if cut != l && strings.Contains(cut, "\x1b[") {
			cut += "\x1b[m" // the cut may drop the run's reset
		}
		lines[i] = cut
	}
	return strings.Join(lines, "\n")
}
