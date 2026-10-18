package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

const (
	helpKeyCol    = 12 // key column width inside a section
	helpColGap    = 4  // spaces between help columns
	helpMaxColumn = 3
)

// helpSection is one titled group of key bindings, rendered as lines.
type helpSection []string

// helpSections is helpSectionsFor the current filter.
func (m *Model) helpSections() []helpSection { return m.helpSectionsFor(m.helpFilter) }

// helpSectionsFor renders every key group that applies to the view help
// was opened from, keeping only the bindings filter matches — all of a
// group whose title it matches — and leaving out groups with none. The sorts, declared among the category's and the view's
// keys, are gathered into one Sort section listed by column.
func (m *Model) helpSectionsFor(filter string) []helpSection {
	var out []helpSection
	filter = strings.ToLower(filter)
	add := func(title string, bind []key.Binding) {
		sec := helpSection{m.theme.HelpHeader.Render(title)}
		whole := strings.Contains(strings.ToLower(title), filter) // "/sort" keeps all of Sort
		for _, b := range bind {
			h := b.Help()
			if h.Key == "" && h.Desc == "" {
				continue
			}
			if !whole && !strings.Contains(strings.ToLower(h.Key+" "+h.Desc), filter) {
				continue
			}
			sec = append(sec, "  "+m.theme.HelpKey.Render(fmt.Sprintf("%-*s", helpKeyCol, h.Key))+
				m.theme.HelpDesc.Render(h.Desc))
		}
		if len(sec) > 1 {
			out = append(out, sec)
		}
	}
	actions, actionSorts := splitSorts(m.keys.Context)
	mode, modeSorts := splitSorts(m.keys.Mode)
	add("Quick switch", m.quickSwitchBindings(m.lastViewMode))
	add("Actions", actions)
	add("Sort", slices.Concat(modeSorts, actionSorts))
	add(fmt.Sprintf("%s view", m.lastViewMode), mode)
	add("General", m.keys.Global)
	switch m.lastViewMode {
	case common.ListView:
		add("Navigation", sentenceCase(m.tableBinding()))
	case common.DetailsView:
		add("Navigation", sentenceCase(m.viewportBinding()))
	case common.LoadingView, common.HelpView, common.ExportView, common.EditTenantView,
		common.LogView, common.ConfirmView, common.RowPickerView, common.PaletteView:
		// No additional sections for these view modes
	}
	return out
}

// splitSorts separates the sort declarations from bind, relabeling each
// sort by its column alone ("s n  Name") for the Sort section.
func splitSorts(bind []key.Binding) (rest, sorts []key.Binding) {
	for _, b := range bind {
		h := b.Help()
		if col, ok := strings.CutPrefix(h.Desc, keys.SortPrefix); ok {
			sorts = append(sorts, key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(h.Key, col)))
			continue
		}
		rest = append(rest, b)
	}
	return rest, sorts
}

// sentenceCase relabels bindings from the bubbles keymaps, whose
// descriptions are lowercase ("up", "½ page down"), to match the
// sentence-case rest of help.
func sentenceCase(bind []key.Binding) []key.Binding {
	out := make([]key.Binding, 0, len(bind))
	for _, b := range bind {
		h := b.Help()
		desc := h.Desc
		if r, n := utf8.DecodeRuneInString(desc); n > 0 {
			desc = string(unicode.ToUpper(r)) + desc[n:]
		}
		out = append(out, key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(h.Key, desc)))
	}
	return out
}

// helpLines lays the sections out in as many columns (up to three) as the
// modal's width allows and returns the resulting body lines.
func (m *Model) helpLines() []string { return m.helpLinesFor(m.helpFilter) }

func (m *Model) helpLinesFor(filter string) []string {
	columns, secW, _ := m.helpColumnsFor(filter)
	return joinHelpColumns(columns, secW)
}

// helpColumns is helpColumnsFor the current filter.
func (m *Model) helpColumns() (columns [][]string, secW int, titles map[string]bool) {
	return m.helpColumnsFor(m.helpFilter)
}

// helpColumnsFor packs filter's sections into columns, returning them with
// their width and the set of section-title lines.
func (m *Model) helpColumnsFor(filter string) (columns [][]string, secW int, titles map[string]bool) {
	secs := m.helpSectionsFor(filter)
	titles = make(map[string]bool, len(secs))
	for _, sec := range secs {
		titles[sec[0]] = true
		for _, l := range sec {
			secW = max(secW, lipgloss.Width(l))
		}
	}
	inner := m.viewWidth*9/10 - 4 // modal border and padding
	cols := min(max((inner+helpColGap)/(secW+helpColGap), 1), helpMaxColumn)
	return packColumns(secs, cols), secW, titles
}

// helpWindowLines is the help body scrolled to off, rows lines tall. A
// section title that would be the window's last line — its keys below it,
// out of view — is left for the next page, so no title shows alone.
func (m *Model) helpWindowLines(off, rows int) []string {
	return m.helpWindowLinesFor(m.helpFilter, off, rows)
}

func (m *Model) helpWindowLinesFor(filter string, off, rows int) []string {
	columns, secW, titles := m.helpColumnsFor(filter)
	window := make([][]string, len(columns))
	for i, col := range columns {
		lo, hi := min(off, len(col)), min(off+rows, len(col))
		window[i] = slices.Clone(col[lo:hi])
		if n := len(window[i]); n == rows && titles[window[i][n-1]] {
			window[i][n-1] = ""
		}
	}
	lines := joinHelpColumns(window, secW)
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines[:rows]
}

// joinHelpColumns sets columns side by side, each secW wide.
func joinHelpColumns(columns [][]string, secW int) []string {
	pad := lipgloss.NewStyle().Width(secW)
	parts := make([]string, 0, 2*len(columns))
	for i, col := range columns {
		if i > 0 {
			parts = append(parts, strings.Repeat(" ", helpColGap))
		}
		parts = append(parts, pad.Render(strings.Join(col, "\n")))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, parts...), "\n")
}

// packColumns fills up to n columns top to bottom, in order, starting a
// new column once the current one would pass an even share of the lines.
// Sections in one column are separated by a blank line.
func packColumns(secs []helpSection, n int) [][]string {
	total := 0
	for _, sec := range secs {
		total += len(sec) + 1
	}
	target := (total + n - 1) / n
	columns := [][]string{nil}
	for _, sec := range secs {
		cur := &columns[len(columns)-1]
		if len(*cur) > 0 && len(*cur)+1+len(sec) > target && len(columns) < n {
			columns = append(columns, nil)
			cur = &columns[len(columns)-1]
		}
		if len(*cur) > 0 {
			*cur = append(*cur, "")
		}
		*cur = append(*cur, sec...)
	}
	return columns
}

// helpWindow is how many body lines help shows at once, and the largest
// scroll offset; maxScroll is 0 when everything fits. The filter line and
// the blank under it take two rows while a filter is typed or set.
func (m *Model) helpWindow(lines int) (rows, maxScroll int) {
	return m.helpWindowFor(lines, m.helpTyping || m.helpFilter != "")
}

func (m *Model) helpWindowFor(lines int, filtering bool) (rows, maxScroll int) {
	body := m.modalMaxBody()
	if filtering {
		body = max(body-2, 1)
	}
	if lines <= body {
		return lines, 0
	}
	rows = max(body-1, 1) // one line for the scroll indicator
	return rows, lines - rows
}

// fullHelpView renders the help modal: key groups in columns, scrolled
// to m.helpScroll with an indicator when they don't fit. Filtered, it
// keeps the size of unfiltered help (fixedModal).
func (m *Model) fullHelpView() string {
	body, maxScroll := m.helpBody(m.helpFilter, m.helpTyping, m.helpScroll)
	full, _ := m.helpBody("", false, 0)
	return m.fixedModal("Keys · "+m.lastViewMode.String()+" view", body, m.helpHints(maxScroll),
		m.theme.Palette.Accent, 0, lipgloss.Width(full), lipgloss.Height(full))
}

// helpBody is help's body for filter (typing while its input is open)
// scrolled to scroll: the head, the window of key groups, and a scroll
// indicator when they don't fit; maxScroll is 0 when they do.
func (m *Model) helpBody(filter string, typing bool, scroll int) (body string, maxScroll int) {
	muted := lipgloss.NewStyle().Foreground(m.theme.Palette.Muted)
	lines := m.helpLinesFor(filter)
	filtering := typing || filter != ""
	matches := len(m.helpSectionsFor(filter)) > 0
	if filtering && !matches {
		lines = []string{muted.Render("no keys match · esc clears the filter")}
	}
	rows, maxScroll := m.helpWindowFor(len(lines), filtering)
	off := min(max(scroll, 0), maxScroll)
	window := lines[off : off+rows]
	if !filtering || matches {
		window = m.helpWindowLinesFor(filter, off, rows)
	}
	body = m.helpHead(strings.Join(window, "\n"), len(lines), maxScroll, filter, typing)
	if maxScroll > 0 {
		var where string
		switch off {
		case 0:
			where = "↓ more"
		case maxScroll:
			where = "↑ more"
		default:
			where = "↑↓ more"
		}
		body += "\n" + muted.Render(fmt.Sprintf("%s · %d%%", where, off*100/maxScroll))
	}
	return body, maxScroll
}

// helpHead tops body with the filter input (typing) or chip, or,
// unfiltered, the logo when it fits without making help scroll.
func (m *Model) helpHead(body string, lines, maxScroll int, filter string, typing bool) string {
	switch {
	case typing:
		return m.helpInput.View() + "\n\n" + body
	case filter != "":
		return lipgloss.NewStyle().Foreground(m.theme.Palette.Accent).Render("⟨/"+filter+"⟩") + "\n\n" + body
	}
	if logoH := lipgloss.Height(view.Logo) + 1; maxScroll == 0 && lines+logoH <= m.modalMaxBody() {
		logo := view.RenderLogo(m.theme.LogoTop, m.theme.LogoBottom)
		return lipgloss.PlaceHorizontal(lipgloss.Width(body), lipgloss.Center, logo) + "\n" + body
	}
	return body
}

// helpHints is help's key line for its state: typing a filter, filtered,
// or not, with the scroll keys when it scrolls.
func (m *Model) helpHints(maxScroll int) string {
	switch {
	case m.helpTyping:
		return "enter keep · esc clear"
	case maxScroll > 0 && m.helpFilter != "":
		return "j/k pgup/pgdn home/end scroll · / filter · esc clear"
	case maxScroll > 0:
		return "j/k pgup/pgdn home/end scroll · / filter · esc close"
	case m.helpFilter != "":
		return "/ filter · esc clear"
	}
	return "/ filter · esc close"
}

// scrollHelp moves the help body by delta lines (or to an end), clamped
// to the content.
func (m *Model) scrollHelp(k string) {
	rows, maxScroll := m.helpWindow(len(m.helpLines()))
	switch k {
	case "j", "down":
		m.helpScroll++
	case "k", "up":
		m.helpScroll--
	case "pgdown", "f", "space":
		m.helpScroll += rows
	case "pgup", "b":
		m.helpScroll -= rows
	case "home":
		m.helpScroll = 0
	case "end":
		m.helpScroll = maxScroll
	}
	m.helpScroll = min(max(m.helpScroll, 0), maxScroll)
}
