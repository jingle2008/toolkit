package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// frameChrome is the rows the frame spends around its panel: the env bar
// and breadcrumb above, the footer below.
const frameChrome = 3

// centered is a thin wrapper around view.CenterText using model dims.
func (m *Model) centered(msg string) string {
	return view.CenterText(msg, m.viewWidth, m.viewHeight)
}

// frame draws the list and details screens: env bar, breadcrumb, main in
// a titled panel, and the footer of key hints.
func (m *Model) frame(title, main string) string {
	p := m.theme.Palette
	titleStyle := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	var panel string
	if m.previewShown() {
		left := view.Panel(title, main, p.Border, titleStyle, m.viewWidth-m.previewOuterWidth())
		panel = lipgloss.JoinHorizontal(lipgloss.Top, left, m.previewPanel(lipgloss.Height(left)-2))
	} else {
		panel = view.Panel(title, main, p.Border, titleStyle, m.viewWidth)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.envBar(), m.statusView(), panel, m.footer())
}

// tableTitle names the list panel: the category, its row count, and how
// many columns are hidden on a terminal too narrow for them.
func (m *Model) tableTitle() string {
	title := fmt.Sprintf("%s (%d)", m.category.String(), len(m.table.Rows()))
	switch m.hiddenColumns {
	case 0:
	case 1:
		title += " · 1 column hidden"
	default:
		title += fmt.Sprintf(" · %d columns hidden", m.hiddenColumns)
	}
	return title
}

// envBar is the frame's first line: where the data comes from
// (realm › type › region · context · k8s, with production's type as a
// red badge so it is never mistaken for dev) on the left; the other
// quick-switch slots when they fit; and the loading spinner, LIVE and the
// version on the right.
func (m *Model) envBar() string {
	p := m.theme.Palette
	muted := lipgloss.NewStyle().Foreground(p.Muted)
	strong := lipgloss.NewStyle().Foreground(p.Text).Bold(true)

	k8s := m.k8sVersion
	if k8s == "" {
		k8s = "…"
	}
	env := m.environment
	envType := strong.Render(env.Type)
	if env.IsProduction() {
		envType = m.theme.Prod.Render(strings.ToUpper(env.Type))
	}
	left := " " + strong.Render(env.Realm) + muted.Render(" › ") + envType +
		muted.Render(" › ") + strong.Render(env.Region) + muted.Render(" · ") + env.KubeContext() +
		muted.Render(" · k8s ") + k8s

	var right []string
	if m.pendingTasks > 0 {
		right = append(right, m.loadingSpinner.View()+" "+m.loadingTimer.View())
	}
	// k8s categories: live while their watch is established (m.watch.k8sActive).
	// repo categories: live while the always-on working-tree watch runs.
	if m.watch.k8sActive || (m.watch.repoActive && !m.category.NeedsKubeConfig()) {
		right = append(right, m.theme.Live.Render("● LIVE"))
	}
	if m.version != "" {
		right = append(right, muted.Render(m.version))
	}
	rightCell := strings.Join(right, "  ") + " "

	w := lipgloss.Width
	room := m.viewWidth - w(rightCell)
	if quick := m.quickSwitchStrip(); quick != "" && w(left)+2+w(quick)+1 <= room {
		left += "  " + quick
	}
	if w(left) > room {
		left = truncateString(ansi.Strip(left), max(room, 1))
	}
	return left + strings.Repeat(" ", max(room-w(left), 0)) + rightCell
}

// quickSwitchStrip lists the other quick-switch slots, e.g. "1 dp-prod-ord",
// so the digit keys stay discoverable without the help screen.
func (m *Model) quickSwitchStrip() string {
	p := m.theme.Palette
	key := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	var parts []string
	for _, b := range m.quickSwitchBindings(m.viewMode) {
		desc := b.Help().Desc
		if strings.HasSuffix(desc, currentSlotMarker) {
			continue
		}
		parts = append(parts, key.Render(b.Keys()[0])+" "+desc)
	}
	return strings.Join(parts, "  ")
}

// statusView is the frame's breadcrumb line: the category path as a chip,
// then either the filter/command input while typing or chips for what
// shapes the rows (filter, sort, faulty-only, history, a pending prefix);
// row stats and the cursor position on the right.
func (m *Model) statusView() string {
	w := lipgloss.Width
	maxCtx := m.viewWidth / 2
	contextCell := m.theme.Context.Render(truncateString(m.contextString(), maxCtx))

	// On a narrow terminal the stats give way to the bare cursor position,
	// then the category chip shrinks, so the line never overflows.
	statsCell := m.theme.Stats.Render(m.statsText())
	if w(contextCell)+w(statsCell) > m.viewWidth {
		statsCell = m.theme.Stats.Render(m.cursorText())
	}
	if over := w(contextCell) + w(statsCell) - m.viewWidth; over > 0 {
		ctx := truncateString(m.contextString(), max(maxCtx-over, 1))
		contextCell = m.theme.Context.Render(ctx)
	}

	var middle string
	if m.inputMode != common.NormalInput {
		// Render-time width depends on the surrounding cells, so compute
		// it here rather than in updateLayout. We deliberately operate on
		// a copy of the textinput so View() stays pure — the original
		// *m.textInput owned by the reducer is never mutated.
		ti := *m.textInput
		ti.SetWidth(max(m.viewWidth-w(contextCell)-w(statsCell)-w(ti.Prompt)-1, 0))
		middle = ti.View()
	} else {
		middle = m.breadcrumbChips()
	}
	room := m.viewWidth - w(contextCell) - w(statsCell)
	if w(middle) > room {
		middle = truncateString(ansi.Strip(middle), max(room, 1))
	}
	return contextCell + middle + strings.Repeat(" ", max(room-w(middle), 0)) + statsCell
}

// statsText is the breadcrumb's right cell: the category's row stats,
// then the cursor position; in details view, the layout shown and how far
// down it is scrolled.
func (m *Model) statsText() string {
	if m.viewMode == common.DetailsView {
		return m.detailPosition()
	}
	var b strings.Builder
	if len(m.stats) > 0 {
		names := make([]string, 0, len(m.stats))
		for k := range m.stats {
			names = append(names, k)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, k := range names {
			parts = append(parts, fmt.Sprintf("%s: %d", k, m.stats[k]))
		}
		b.WriteString(strings.Join(parts, " • "))
		b.WriteString(" ")
	}
	b.WriteString(m.cursorText())
	return b.String()
}

// detailPosition is the details view's layout and, when it doesn't all
// fit, how far down it is scrolled: "[fields]", "[JSON · 40%]".
func (m *Model) detailPosition() string {
	layout := "fields"
	if m.detailJSON {
		layout = "JSON"
	}
	if m.viewport.TotalLineCount() <= m.viewport.Height() {
		return "[" + layout + "]"
	}
	return fmt.Sprintf("[%s · %d%%]", layout, int(m.viewport.ScrollPercent()*100))
}

// cursorText is the cursor position among the rows, e.g. "[12/124]".
func (m *Model) cursorText() string {
	return fmt.Sprintf("[%d/%d]", m.table.Cursor()+1, len(m.table.Rows()))
}

// breadcrumbChips shows, only when active, what narrows or orders the
// rows: the filter, a non-default sort, faulty-only mode, the position in
// navigation history, and a pending prefix key.
func (m *Model) breadcrumbChips() string {
	p := m.theme.Palette
	chip := lipgloss.NewStyle().Foreground(p.Secondary)
	var chips []string
	if m.filter != "" {
		chips = append(chips, chip.Render("⟨/"+m.filter+"⟩"))
	}
	if m.sortColumn != "" && (m.sortColumn != common.NameCol || !m.sortAsc) {
		dir := "▲"
		if !m.sortAsc {
			dir = "▼"
		}
		chips = append(chips, chip.Render("⟨sort: "+strings.ToLower(m.sortColumn)+" "+dir+"⟩"))
	}
	if m.showFaulty {
		chips = append(chips, lipgloss.NewStyle().Foreground(p.Danger).Render("⟨faulty⟩"))
	}
	if len(m.history) > 1 {
		chips = append(chips, chip.Render(fmt.Sprintf("⟨◀ %d/%d⟩", m.historyIdx+1, len(m.history))))
	}
	if cell := m.prefixCell(); cell != "" {
		chips = append(chips, cell)
	}
	if len(chips) == 0 {
		return ""
	}
	return "  " + strings.Join(chips, " ")
}

// footer is the frame's last line: the category's most useful actions,
// then the navigation keys (copy, filter, palette) and "? more". Actions
// drop from the right to fit before any navigation key does.
func (m *Model) footer() string {
	p := m.theme.Palette
	keyStyle := lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(p.Muted)
	items := func(bindings []key.Binding) []string {
		out := make([]string, 0, len(bindings))
		for _, b := range bindings {
			out = append(out, keyStyle.Render(b.Help().Key)+" "+descStyle.Render(lowerFirst(b.Help().Desc)))
		}
		return out
	}

	actionBindings, coreBindings := m.keys.FooterBindings()
	actions, core := items(actionBindings), items(coreBindings)
	more := keyStyle.Render("?") + " " + descStyle.Render("more")
	fits := func(parts []string) (string, bool) {
		line := " " + strings.Join(append(parts, more), "  ")
		return line, lipgloss.Width(line) <= m.viewWidth
	}
	for n := len(actions); n >= 0; n-- {
		if line, ok := fits(append(actions[:n:n], core...)); ok {
			return line
		}
	}
	for n := len(core) - 1; n >= 0; n-- {
		if line, ok := fits(core[:n:n]); ok {
			return line
		}
	}
	return ""
}

// lowerFirst lowercases s's first letter, so a binding's sentence-case
// description ("Copy tenant ID") reads as a footer phrase ("copy tenant
// ID").
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// loadingView is the first-load screen: the logo over the spinner, the
// category being loaded and the elapsed time.
func (m *Model) loadingView() string {
	logo := view.RenderLogo(m.theme.LogoTop, m.theme.LogoBottom)
	line := fmt.Sprintf("%s Loading %s … %s", m.loadingSpinner.View(), m.category.String(), m.loadingTimer.View())
	return m.centered(lipgloss.JoinVertical(lipgloss.Center, logo, "", line))
}
