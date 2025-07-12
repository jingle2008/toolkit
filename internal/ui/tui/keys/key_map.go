// Package keys provides key binding definitions and utilities for the TUI.
package keys

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap holds key bindings for the TUI, composed of global, mode, and context (category+mode) keys.
type KeyMap struct {
	Global  []key.Binding // always active
	Mode    []key.Binding // active for current UI mode
	Context []key.Binding // category-specific and mode-specific (optional)
}

// ShortHelp returns a short list of key bindings for help display.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help(), k.Quit()}
}

// FullHelp returns a full list of key bindings for help display, chunked per category.
func (k KeyMap) FullHelp() [][]key.Binding {
	cat := slices.Concat(k.Global, k.Mode, k.Context)
	slices.SortFunc(cat, func(c1, c2 key.Binding) int {
		return strings.Compare(c1.Help().Desc, c2.Help().Desc)
	})

	rows := [][]key.Binding{}
	for chunk := range slices.Chunk(cat, 5) {
		rows = append(rows, chunk)
	}
	return rows
}

// Help returns the help key binding from the global keys.
func (k KeyMap) Help() key.Binding {
	return findBindingByHelp(k.Global, "Help")
}

// Quit returns the quit key binding from the global keys.
func (k KeyMap) Quit() key.Binding {
	return findBindingByHelp(k.Global, "Quit")
}

// findBindingByHelp finds a key.Binding in the slice by its help text.
func findBindingByHelp(bindings []key.Binding, help string) key.Binding {
	for _, b := range bindings {
		if b.Help().Key == help || b.Help().Desc == help {
			return b
		}
	}
	return key.Binding{}
}
