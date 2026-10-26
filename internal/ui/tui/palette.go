// Package tui — command palette.
//
// The palette itself (query, matching, cursor, rendering) lives in the
// palette sub-package; this file is the parent's glue: building the items
// from the current category, environment and key map together with what
// each one runs (closures over the live Model, invoked synchronously on
// choose), opening and closing the palette, the quit key, dispatching a
// palette.Result, the session's recent list, and the theme-derived
// styles and size the palette draws in.
package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/internal/ui/tui/palette"
	"github.com/jingle2008/toolkit/pkg/models"
)

// paletteItem is one thing the palette can run: what it shows, and run,
// invoked on the update goroutine when it is chosen.
type paletteItem struct {
	palette.Item
	run func() tea.Cmd
}

// palettePending is the parent's half of the open palette: what each
// item runs, by its index among the palette's items, and the view to
// restore on close.
type palettePending struct {
	runs       []func() tea.Cmd
	returnView common.ViewMode
}

// openPalette shows the palette over the current view with every item
// listed, ready for a query.
func (m *Model) openPalette() {
	all := m.paletteItems()
	items := make([]palette.Item, len(all))
	runs := make([]func() tea.Cmd, len(all))
	for i, it := range all {
		items[i], runs[i] = it.Item, it.run
	}
	m.palette = palette.New(items, m.paletteRecent)
	m.palettePending = palettePending{runs: runs, returnView: m.viewMode}
	m.viewMode = common.PaletteView
}

// paletteItems lists what the palette offers right now: every category
// (with its aliases), the environments when the category is
// environment-scoped (recent slots first, then the rest), the selected
// row's actions and the category's sorts, and the toggles.
func (m *Model) paletteItems() []paletteItem {
	var items []paletteItem
	for _, e := range domain.AliasEntries() {
		cat, err := domain.ParseCategory(e.Name)
		if err != nil {
			continue
		}
		items = append(items, paletteItem{
			Item: palette.Item{Kind: palette.KindCategory, Label: e.Name, Hint: m.categoryHint(cat, e.Aliases)},
			run:  func() tea.Cmd { return m.openCategory(cat) },
		})
	}
	items = append(items, m.paletteEnvironments()...)
	items = append(items, m.paletteActions()...)
	items = append(items,
		paletteItem{
			Item: palette.Item{Kind: "toggle", Label: "Alias list", Hint: keys.ToggleAlias.Help().Key},
			run:  func() tea.Cmd { return tea.Batch(m.toggleAliases()...) },
		},
		paletteItem{
			Item: palette.Item{Kind: "toggle", Label: "Preview pane", Hint: keys.TogglePreview.Help().Key},
			run:  m.togglePreview,
		})
	return items
}

// categoryHint is a category's short aliases (the full lowercase name the
// label already says is left out), marked when it is the one open.
func (m *Model) categoryHint(cat domain.Category, aliases []string) string {
	short := make([]string, 0, len(aliases))
	for _, a := range aliases {
		if a != strings.ToLower(cat.String()) {
			short = append(short, a)
		}
	}
	hint := strings.Join(short, " ")
	if cat == m.category {
		hint += currentSlotMarker
	}
	return strings.TrimSpace(hint)
}

// paletteEnvironments offers the environments the quick-switch keys can
// reach — only in environment-scoped categories, as quick switch is.
func (m *Model) paletteEnvironments() []paletteItem {
	if !m.category.IsEnvironmentScoped() {
		return nil
	}
	var envs []models.Environment
	for _, e := range m.recentEnvs.BySlot() {
		envs = append(envs, e.Env)
	}
	if m.dataset != nil {
		for _, env := range m.dataset.Environments {
			if !slices.ContainsFunc(envs, env.Equals) {
				envs = append(envs, env)
			}
		}
	}
	items := make([]paletteItem, 0, len(envs))
	for _, env := range envs {
		hint := env.Realm + " " + env.Region
		if env.Equals(m.environment) {
			hint += currentSlotMarker
		}
		items = append(items, paletteItem{
			Item: palette.Item{Kind: "env", Label: env.KubeContext(), Hint: hint},
			run: func() tea.Cmd {
				if env.Equals(m.environment) {
					return nil
				}
				return m.switchEnvironment(env, m.category)
			},
		})
	}
	return items
}

// paletteActions offers the list view's per-row actions and toggles (as
// their keys would run them), each sort, and Export CSV. The palette only
// opens from the list view (: is a list-view key).
func (m *Model) paletteActions() []paletteItem {
	var items []paletteItem
	sortItem := func(h key.Help) (paletteItem, bool) {
		col, ok := strings.CutPrefix(h.Desc, keys.SortPrefix)
		return paletteItem{
			Item: palette.Item{Kind: "action", Label: "Sort by " + col, Hint: h.Key},
			run:  func() tea.Cmd { return m.sortTableByColumn(col) },
		}, ok
	}
	// Sorts every list view has (Name) are mode keys, not the category's.
	for _, b := range m.keys.Mode {
		if it, ok := sortItem(b.Help()); ok {
			items = append(items, it)
		}
	}
	for _, b := range m.keys.Context {
		h := b.Help()
		if it, ok := sortItem(h); ok {
			items = append(items, it)
			continue
		}
		kind := "action"
		if key.Matches(pressFor(b.Keys()[0]), keys.ToggleFaulty) {
			kind = "toggle"
		}
		k := b.Keys()[0]
		items = append(items, paletteItem{
			Item: palette.Item{Kind: kind, Label: h.Desc, Hint: h.Key},
			run:  func() tea.Cmd { return tea.Batch(m.handleNormalKeys(pressFor(k))...) },
		})
	}
	items = append(items, paletteItem{
		Item: palette.Item{Kind: "action", Label: "Export CSV", Hint: keys.ExportCSV.Help().Key},
		run:  func() tea.Cmd { return tea.Batch(m.enterExportView()...) },
	})
	return items
}

// openCategory navigates to cat, as typing its alias after : used to.
func (m *Model) openCategory(cat domain.Category) tea.Cmd {
	if m.category == cat {
		return nil
	}
	return tea.Sequence(m.updateCategory(cat)...)
}

// pressFor builds the key event for a binding's key ("o", "R", "ctrl+x"),
// so a palette action runs exactly as pressing its key would.
func pressFor(k string) tea.KeyPressMsg {
	if c, ok := strings.CutPrefix(k, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(c)[0], Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

// updatePaletteView resolves a message while the palette is open: the
// palette decides (palette.Model.Update), and a choice runs, a close
// closes. ctrl+c is the parent's: it quits from the palette.
func (m *Model) updatePaletteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok && km.String() == "ctrl+c" {
		return m, m.quit()
	}
	var res palette.Result
	var cmd tea.Cmd
	m.palette, res, cmd = m.palette.Update(msg, m.paletteRows())
	switch res.Outcome {
	case palette.Chosen:
		return m, m.runPalette(res)
	case palette.Closed:
		m.closePalette()
	case palette.None:
	}
	return m, cmd
}

// runPalette closes the palette and runs its choice: the category an
// exact alias named, else the chosen item.
func (m *Model) runPalette(res palette.Result) tea.Cmd {
	m.rememberPalette(res.Item)
	if res.Index < 0 {
		m.closePalette()
		return m.openCategory(res.Category)
	}
	run := m.palettePending.runs[res.Index]
	m.closePalette()
	return run()
}

// rememberPalette puts it first in the recent list.
func (m *Model) rememberPalette(it palette.Item) {
	m.paletteRecent = palette.Remember(m.paletteRecent, it)
}

func (m *Model) closePalette() {
	m.viewMode = m.palettePending.returnView
	m.palette = palette.Model{}
	m.palettePending = palettePending{}
}

// paletteRows is how many matches the palette lists before scrolling.
func (m *Model) paletteRows() int {
	return max(min(palette.MaxRows, m.modalMaxBody()-3), 1) // query, blank, indicator
}

// paletteView renders the palette in the theme, sized to the screen.
func (m *Model) paletteView() string {
	p := m.theme.Palette
	return m.palette.View(m.modalMaxWidth(), m.paletteRows(), palette.Styles{Accent: p.Accent, Muted: p.Muted})
}
