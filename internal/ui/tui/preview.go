package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/pkg/models"
)

// previewMinWidth is the narrowest terminal that shows the preview pane;
// below it the table needs every column.
const previewMinWidth = 160

// previewShown reports whether the list view draws the preview pane now:
// toggled on, in list view, on a wide enough terminal.
func (m *Model) previewShown() bool {
	return m.preview && m.viewMode == common.ListView && m.viewWidth >= previewMinWidth
}

// previewOuterWidth is the preview panel's width, borders included: 40%
// of the terminal.
func (m *Model) previewOuterWidth() int {
	return m.viewWidth * 2 / 5
}

// togglePreview flips the preview pane for the session. On a terminal too
// narrow for it, it says so instead.
func (m *Model) togglePreview() tea.Cmd {
	if !m.preview && m.viewWidth < previewMinWidth {
		return m.showToast(fmt.Sprintf("preview needs ≥%d columns (have %d)", previewMinWidth, m.viewWidth), toastInfo)
	}
	m.preview = !m.preview
	m.updateLayout(m.viewWidth, m.viewHeight)
	return nil
}

// previewPanel renders the selected row's summary in a panel titled with
// its name, height rows tall to match the table's panel.
func (m *Model) previewPanel(height int) string {
	p := m.theme.Palette
	width := m.previewOuterWidth()
	row := m.selectedRawRow()
	title := ""
	if len(row) > 0 {
		title = row[0]
	}
	body := m.previewBody(width - 4) // borders and padding
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return view.Box(title, strings.Join(lines, "\n"), p.Border,
		lipgloss.NewStyle().Foreground(p.Text).Bold(true), width)
}

// previewCache memoizes the preview body, which View would otherwise
// rebuild (find, marshal, parse, lay out) on every frame, spinner ticks
// included. The key is everything the body depends on; a different key
// is a miss.
type previewCache struct {
	key  previewKey
	body string
	ok   bool
}

// previewKey is what the preview body is a function of. The dataset is
// keyed by identity: every update installs a new *Dataset rather than
// editing the old one (editDataset), so a new pointer is a new
// generation; holding it keeps its address from being reused. The theme
// is keyed by its generation (themeGen).
type previewKey struct {
	dataset  *models.Dataset
	category domain.Category
	item     models.ItemKey // a string, a ScopedItemKey or nil: comparable
	width    int
	theme    int
}

// previewBody is the selected item's fields, width columns wide: one
// level of nested objects as indented sections, deeper ones collapsed,
// each value cut to one line. It is rebuilt only when what it shows
// changes (previewCache).
func (m *Model) previewBody(width int) string {
	key := previewKey{
		dataset:  m.dataset,
		category: m.category,
		item:     itemKeyFrom(m.category, m.selectedRawRow()),
		width:    width,
		theme:    m.themeGen,
	}
	if c := &m.previewMemo; c.ok && c.key == key {
		return c.body
	}
	body := m.renderPreviewBody(key.item, width)
	m.previewMemo = previewCache{key: key, body: body, ok: true}
	return body
}

// renderPreviewBody lays out the keyed item of the current category.
func (m *Model) renderPreviewBody(key models.ItemKey, width int) string {
	muted := lipgloss.NewStyle().Foreground(m.theme.Palette.Muted)
	item := findItem(m.dataset, m.category, key)
	if item == nil {
		return muted.Render("nothing selected")
	}
	tree, err := itemFields(item)
	if err != nil {
		return muted.Render(err.Error())
	}
	return renderFields(tree, width, 1, false, false, m.fieldStyles())
}

// itemFields is item's JSON, as the details view and copy show it, as a
// node tree.
func itemFields(item any) (node, error) {
	content, err := jsonutil.Pretty(item)
	if err != nil {
		return node{}, err
	}
	return parseFields([]byte(content))
}

// fieldStyles colors a fields layout from the theme.
func (m *Model) fieldStyles() fieldStyles {
	p := m.theme.Palette
	return fieldStyles{
		label:   lipgloss.NewStyle().Foreground(p.Muted),
		section: lipgloss.NewStyle().Foreground(p.Text).Bold(true),
	}
}
