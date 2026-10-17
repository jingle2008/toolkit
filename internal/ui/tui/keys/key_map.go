// Package keys provides key binding definitions and utilities for the TUI.
package keys

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
)

// KeyMap holds key bindings for the TUI, composed of global, mode, and context (category+mode) keys.
type KeyMap struct {
	Global  []key.Binding // always active
	Mode    []key.Binding // active for current UI mode
	Context []key.Binding // category-specific and mode-specific (optional)
	// QuickSwitch holds one binding per occupied recent-context slot, in
	// slot order. Filled at render time by the model; ResolveKeys leaves
	// it empty.
	QuickSwitch []key.Binding
}

// ShortHelp returns a short list of key bindings for help display.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help(), k.Quit()}
}

// SortableColumns returns the set of column titles reachable via a
// Sort* binding in this KeyMap. The header renderer uses it to mark
// sortable-but-not-currently-active columns with a ↕ glyph.
//
// Keys are lowercased so callers can do case-insensitive lookups
// (`sortable[strings.ToLower(title)]`), matching the EqualFold
// comparison the active-sort arrow renderer uses against m.sortColumn.
// Global is excluded by design — no Sort* binding is expected to live
// there.
func (k KeyMap) SortableColumns() map[string]bool {
	out := map[string]bool{}
	for _, group := range [][]key.Binding{k.Mode, k.Context} {
		for _, b := range group {
			if col, ok := strings.CutPrefix(b.Help().Desc, SortPrefix); ok {
				out[strings.ToLower(col)] = true
			}
		}
	}
	return out
}

// FullHelp returns a full list of key bindings for help display, chunked per category.
// QuickSwitch bindings, when present, form their own leading column in slot
// order instead of joining the alphabetical sort.
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
	if len(k.QuickSwitch) > 0 {
		rows = append(rows, k.QuickSwitch)
	}
	for chunk := range slices.Chunk(cat, helpRows) {
		rows = append(rows, chunk)
	}
	return rows
}

// helpRows is the height of one help column; it matches the six-line info
// block the header sits beside.
const helpRows = 6

// headerCore lists the non-context bindings the header bar keeps beside a
// category's actions: the universal entry points, plus details view's one
// action. Each is shown only if the active KeyMap carries it. Everything
// else (navigation, history, toggles) lives on the ? screen. Help is not
// listed here — HeaderHelp pins it last.
var headerCore = []key.Binding{CopyObject, FilterMode, CommandMode, Back}

/*
HeaderHelp returns the header bar's help columns, fitted to width.

Bindings are ranked rather than sorted globally: category actions
(Context, minus Sort* keys, which the column headers already advertise)
first, then the headerCore keys this KeyMap carries, each tier sorted by
description. When the columns don't fit, bindings are dropped from the
tail — core before actions — and the pinned Help entry reports how many
were hidden, so overflow is never silent. QuickSwitch keeps its leading
column; if even QuickSwitch and Help overflow, nothing is returned.
measure reports the rendered width of a column set; width <= 0 disables
fitting.
*/
func (k KeyMap) HeaderHelp(width int, measure func([][]key.Binding) int) [][]key.Binding {
	byDesc := func(c1, c2 key.Binding) int {
		return strings.Compare(c1.Help().Desc, c2.Help().Desc)
	}
	actions := slices.DeleteFunc(slices.Clone(k.Context), func(b key.Binding) bool {
		return strings.HasPrefix(b.Help().Desc, SortPrefix)
	})
	slices.SortStableFunc(actions, byDesc)

	active := slices.Concat(k.Global, k.Mode)
	var core []key.Binding
	for _, c := range headerCore {
		if slices.ContainsFunc(active, func(b key.Binding) bool { return b.Help() == c.Help() }) {
			core = append(core, c)
		}
	}
	slices.SortStableFunc(core, byDesc)

	items := slices.Concat(actions, core)
	for n := len(items); n >= 0; n-- {
		cols := k.headerColumns(items[:n], len(items)-n)
		if width <= 0 || measure(cols) <= width {
			return cols
		}
	}
	// Not even Help fits. Return nothing: handing bubbles an oversized
	// column makes it skip its ellipsis and render the column anyway.
	return nil
}

// headerColumns lays out the QuickSwitch column, then items followed by
// the Help binding, chunked into helpRows-tall columns. hidden > 0 tags
// Help with the number of bindings that were dropped to fit.
func (k KeyMap) headerColumns(items []key.Binding, hidden int) [][]key.Binding {
	if h := k.Help(); h.Enabled() {
		if hidden > 0 {
			hh := h.Help()
			h = key.NewBinding(
				key.WithKeys(h.Keys()...),
				key.WithHelp(hh.Key, fmt.Sprintf("%s +%d more", hh.Desc, hidden)),
			)
		}
		items = append(slices.Clone(items), h)
	}
	cols := [][]key.Binding{}
	if len(k.QuickSwitch) > 0 {
		cols = append(cols, k.QuickSwitch)
	}
	for chunk := range slices.Chunk(items, helpRows) {
		cols = append(cols, chunk)
	}
	return cols
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
