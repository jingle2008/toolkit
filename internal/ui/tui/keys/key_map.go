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
	// skip sort keys
	cat = slices.DeleteFunc(cat, func(b key.Binding) bool {
		return strings.HasPrefix(b.Help().Desc, SortPrefix)
	})
	slices.SortFunc(cat, func(c1, c2 key.Binding) int {
		return strings.Compare(c1.Help().Desc, c2.Help().Desc)
	})

	rows := [][]key.Binding{}
	for chunk := range slices.Chunk(cat, 5) {
		rows = append(rows, chunk)
	}
	return rows
}

/*
Help returns the help key binding from the global keys.
It matches by key ("h") or help text ("help"), case-insensitive.
*/
func (k KeyMap) Help() key.Binding {
	for _, b := range k.Global {
		h := b.Help()
		if strings.EqualFold(h.Key, "h") || strings.EqualFold(h.Desc, "help") {
			return b
		}
	}
	return key.Binding{}
}

/*
Quit returns the quit key binding from the global keys.
It matches by key ("q") or help text ("quit"), case-insensitive.
*/
func (k KeyMap) Quit() key.Binding {
	for _, b := range k.Global {
		h := b.Help()
		if strings.EqualFold(h.Key, "q") || strings.EqualFold(h.Desc, "quit") {
			return b
		}
	}
	return key.Binding{}
}
