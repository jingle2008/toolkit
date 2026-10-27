// Package keys provides key binding definitions and utilities for the TUI.
package keys

import (
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
			if col, ok := SortColumnOf(b); ok {
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
	cat = slices.DeleteFunc(cat, isSort)
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

// helpRows is the height of one FullHelp column.
const helpRows = 6

// footerCore lists the non-context bindings the footer keeps after a
// category's actions, each shown only if the active KeyMap carries it.
var footerCore = []key.Binding{CopyObject, ToggleJSON, FilterMode, CommandMode}

// sortHint stands for every sort declaration in the footer: the column
// headers show the individual keys once s is pressed.
var sortHint = key.NewBinding(key.WithKeys(SortLeader), key.WithHelp(SortLeader, "sort"))

/*
FooterBindings returns the bindings the footer advertises, most useful
first. actions are the category's actions in registry order (sort
declarations collapse into one "s sort"); core are the footerCore keys
this KeyMap carries. The caller appends help and drops actions from the
tail to fit before it drops any core key.
*/
func (k KeyMap) FooterBindings() (actions, core []key.Binding) {
	sortable := false
	for _, b := range slices.Concat(k.Context, k.Mode) {
		if isSort(b) {
			sortable = true
		}
	}
	for _, b := range k.Context {
		if !isSort(b) {
			actions = append(actions, b)
		}
	}
	if sortable {
		actions = append(actions, sortHint)
	}
	active := slices.Concat(k.Global, k.Mode)
	for _, c := range footerCore {
		if slices.ContainsFunc(active, func(b key.Binding) bool { return Same(b, c) }) {
			core = append(core, c)
		}
	}
	return actions, core
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
