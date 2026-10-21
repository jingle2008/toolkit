package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// paletteMaxRows caps how many items the palette lists before scrolling.
const paletteMaxRows = 12

// paletteRecentMax is how many recently run items the palette lists first.
const paletteRecentMax = 5

// paletteItem is one thing the palette can run: a category to open, an
// environment to switch to, an action on the selected row, or a toggle.
type paletteItem struct {
	kind  string // "category", "env", "action" or "toggle", shown on the right
	label string
	hint  string // aliases, or the item's key, shown muted beside the kind
	run   func() tea.Cmd
}

// id names the item across palette openings, for the recent list.
func (it paletteItem) id() string { return it.kind + "\x00" + it.label }

// paletteState is the open command palette. matches holds the items the
// query keeps, best first; cursor and offset index it. With no query the
// first recent matches are the recently run items.
type paletteState struct {
	input      textinput.Model
	items      []paletteItem
	matches    []fuzzy.Match
	recent     int
	cursor     int
	offset     int
	returnView common.ViewMode
}

// openPalette shows the palette over the current view with every item
// listed, ready for a query.
func (m *Model) openPalette() {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.Focus()
	m.palette = paletteState{input: ti, items: m.paletteItems(), returnView: m.viewMode}
	m.refilterPalette()
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
			kind: "category", label: e.Name, hint: m.categoryHint(cat, e.Aliases),
			run: func() tea.Cmd { return m.openCategory(cat) },
		})
	}
	items = append(items, m.paletteEnvironments()...)
	items = append(items, m.paletteActions()...)
	items = append(items,
		paletteItem{
			kind: "toggle", label: "Alias view", hint: keys.ToggleAlias.Help().Key,
			run: func() tea.Cmd { return tea.Batch(m.toggleAliases()...) },
		},
		paletteItem{
			kind: "toggle", label: "Preview pane", hint: keys.TogglePreview.Help().Key,
			run: m.togglePreview,
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
			kind: "env", label: env.KubeContext(), hint: hint,
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
	for _, b := range m.keys.Context {
		h := b.Help()
		if col, ok := strings.CutPrefix(h.Desc, keys.SortPrefix); ok {
			items = append(items, paletteItem{
				kind: "action", label: "Sort by " + col, hint: h.Key,
				run: func() tea.Cmd { return m.sortTableByColumn(col) },
			})
			continue
		}
		kind := "action"
		if key.Matches(pressFor(b.Keys()[0]), keys.ToggleFaulty) {
			kind = "toggle"
		}
		k := b.Keys()[0]
		items = append(items, paletteItem{
			kind: kind, label: h.Desc, hint: h.Key,
			run: func() tea.Cmd { return tea.Batch(m.handleNormalKeys(pressFor(k))...) },
		})
	}
	items = append(items, paletteItem{
		kind: "action", label: "Export CSV", hint: keys.ExportCSV.Help().Key,
		run: func() tea.Cmd { return tea.Batch(m.enterExportView()...) },
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

// refilterPalette re-ranks the items for the current query: every item
// in order when it is empty. Otherwise the category an exact alias names
// comes first (it is what enter runs), then fuzzy matches on labels, then
// items found only through their hint (a category by one of its
// aliases). Labels and hints are matched separately so a match never
// strings letters across both.
func (m *Model) refilterPalette() {
	p := &m.palette
	p.cursor, p.offset = 0, 0
	q := strings.TrimSpace(p.input.Value())
	p.matches = make([]fuzzy.Match, 0, len(p.items))
	p.recent = 0
	seen := make(map[int]bool, len(p.items))
	add := func(mt fuzzy.Match) {
		if !seen[mt.Index] {
			seen[mt.Index] = true
			p.matches = append(p.matches, mt)
		}
	}
	if q == "" {
		for _, id := range m.paletteRecent {
			if i := slices.IndexFunc(p.items, func(it paletteItem) bool { return it.id() == id }); i >= 0 {
				add(fuzzy.Match{Index: i})
			}
		}
		p.recent = len(p.matches)
		for i := range p.items {
			add(fuzzy.Match{Index: i})
		}
		return
	}
	if cat, err := domain.ParseCategory(q); err == nil {
		for i, it := range p.items {
			if it.kind == "category" && it.label == cat.String() {
				add(fuzzy.Match{Index: i})
			}
		}
	}
	labels := make([]string, len(p.items))
	hints := make([]string, len(p.items))
	for i, it := range p.items {
		labels[i], hints[i] = it.label, it.hint
	}
	for _, mt := range fuzzy.Find(q, labels) {
		add(mt)
	}
	for _, mt := range fuzzy.Find(q, hints) {
		mt.MatchedIndexes = nil // they index the hint, not the label
		add(mt)
	}
}

// updatePaletteView moves through the matches (↑/↓, ctrl+p/ctrl+n), runs
// one on enter, closes on esc, and feeds everything else, pastes
// included, to the query. An exact category alias wins on enter whatever
// the ranking, so :dac⏎ still opens DedicatedAICluster.
func (m *Model) updatePaletteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		return m, m.updatePaletteQuery(paste)
	}
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	p := &m.palette
	switch km.String() {
	case "ctrl+c":
		return m, m.quit()
	case "esc":
		m.closePalette()
		return m, nil
	case "up", "ctrl+p":
		p.cursor = max(p.cursor-1, 0)
	case "down", "ctrl+n":
		p.cursor = min(p.cursor+1, max(len(p.matches)-1, 0))
	case "enter":
		return m, m.runPalette()
	default:
		return m, m.updatePaletteQuery(msg)
	}
	p.offset = scrollWindow(p.cursor, p.offset, m.paletteRows())
	return m, nil
}

// updatePaletteQuery feeds msg (a key or a paste) to the query and
// re-ranks the matches when it changed.
func (m *Model) updatePaletteQuery(msg tea.Msg) tea.Cmd {
	p := &m.palette
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		m.refilterPalette()
	}
	return cmd
}

// runPalette closes the palette and runs the exact alias, else the
// highlighted match. With no match it does nothing and stays open, so the
// query can be edited.
func (m *Model) runPalette() tea.Cmd {
	p := m.palette
	if cat, err := domain.ParseCategory(p.input.Value()); err == nil {
		m.rememberPalette(paletteItem{kind: "category", label: cat.String()})
		m.closePalette()
		return m.openCategory(cat)
	}
	if len(p.matches) == 0 {
		return nil
	}
	it := p.items[p.matches[p.cursor].Index]
	m.rememberPalette(it)
	m.closePalette()
	return it.run()
}

// rememberPalette puts it first in the recent list.
func (m *Model) rememberPalette(it paletteItem) {
	id := it.id()
	m.paletteRecent = slices.DeleteFunc(m.paletteRecent, func(r string) bool { return r == id })
	m.paletteRecent = append([]string{id}, m.paletteRecent...)
	if len(m.paletteRecent) > paletteRecentMax {
		m.paletteRecent = m.paletteRecent[:paletteRecentMax]
	}
}

func (m *Model) closePalette() {
	m.viewMode = m.palette.returnView
	m.palette = paletteState{}
}

// paletteRows is how many matches the palette lists before scrolling.
func (m *Model) paletteRows() int {
	return max(min(paletteMaxRows, m.modalMaxBody()-3), 1) // query, blank, indicator
}

// paletteView renders the palette: the query, then one row per visible
// match with its matched characters highlighted, its hint and its group.
// A group (recent, or the item's kind) is named on its first row only.
func (m *Model) paletteView() string {
	p := m.palette
	pal := m.theme.Palette
	width := min(m.modalMaxWidth(), 72)
	inner := width - 4
	n := m.paletteRows()
	start := min(p.offset, max(len(p.matches)-n, 0))
	end := min(start+n, len(p.matches))

	muted := lipgloss.NewStyle().Foreground(pal.Muted)
	hit := lipgloss.NewStyle().Foreground(pal.Accent).Bold(true)
	group := func(i int) string {
		if i < p.recent {
			return "recent"
		}
		return p.items[p.matches[i].Index].kind
	}
	kindW := 0
	for i := range p.matches {
		kindW = max(kindW, len(group(i)))
	}
	lines := []string{p.input.View(), ""}
	for i := start; i < end; i++ {
		mt := p.matches[i]
		it := p.items[mt.Index]
		marker := "  "
		if i == p.cursor {
			marker = hit.Render("▸ ")
		}
		label := highlight(it.label, mt.MatchedIndexes, hit)
		kind := ""
		if i == start || group(i) != group(i-1) {
			kind = group(i)
		}
		right := muted.Render(truncateString(it.hint, inner/3) + "  " + fmt.Sprintf("%-*s", kindW, kind))
		gap := max(inner-2-lipgloss.Width(label)-lipgloss.Width(right), 1)
		lines = append(lines, marker+label+strings.Repeat(" ", gap)+right)
	}
	if len(p.matches) == 0 {
		lines = append(lines, muted.Render("  no matches"))
	}
	if len(p.matches) > n {
		lines = append(lines, muted.Render(fmt.Sprintf("  %d–%d of %d", start+1, end, len(p.matches))))
	}
	// As tall as the unfiltered list: the query, a blank, the rows, and the
	// count line when they scroll.
	bodyH := 2 + min(n, len(p.items))
	if len(p.items) > n {
		bodyH++
	}
	return m.fixedModal("Command palette", strings.Join(lines, "\n"),
		"↑/↓ select · enter run · esc close", pal.Accent, width, 0, bodyH)
}

// highlight renders s with the runes at byte offsets idx in style hit.
func highlight(s string, idx []int, hit lipgloss.Style) string {
	if len(idx) == 0 {
		return s
	}
	marked := make(map[int]bool, len(idx))
	for _, i := range idx {
		marked[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if marked[i] {
			b.WriteString(hit.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
