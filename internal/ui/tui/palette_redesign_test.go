package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// The selected row carries the marker in its left padding, so its text
// lines up with the other rows, and no tag is left in the output.
func TestMarkedTableView_MarkerInPadding(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.DedicatedAICluster)
	m.table.MoveDown(1)
	tableBefore := m.table.View()
	view := m.markedTableView()
	assert.NotContains(t, view, rowTag)
	assert.Equal(t, tableBefore, m.table.View(), "View leaves the table's rendering alone")
	assert.Equal(t, view, m.markedTableView(), "the same frame twice")
	lines := strings.Split(ansi.Strip(view), "\n")
	var sel, other string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, selectionMarker):
			sel = l
		case strings.Contains(l, "dac-alpha"):
			other = l
		}
	}
	require.NotEmpty(t, sel, "a row starts with the marker")
	assert.Contains(t, sel, "dac-orphan")
	col := func(l, name string) int {
		before, _, ok := strings.Cut(l, name)
		require.True(t, ok, "%q in %q", name, l)
		return ansi.StringWidth(before)
	}
	assert.Equal(t, col(other, "dac-alpha"), col(sel, "dac-orphan"), "aligned with the rows around it")
	assert.Equal(t, 1, strings.Count(strings.Join(lines, "\n"), selectionMarker))

	// The table is left as it was.
	assert.Equal(t, "dac-orphan", ansi.Strip(m.table.Rows()[1][0]))
}

// The selected row has no foreground of its own, so status colors and
// the terminal's text color show through on the Selection tint.
func TestSelectedRow_TintWithoutForeground(t *testing.T) {
	t.Parallel()
	s := DefaultStyles(true)
	assert.Equal(t, s.Palette.Selection, s.Table.Selected.GetBackground())
	assert.NotContains(t, s.Table.Selected.Render("x"), "38;", "no foreground set")
}

// A section title that would close the help window waits for the next
// page instead of showing without its keys.
func TestHelpWindow_NoTitleAlone(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 110, 30)
	showCategory(t, m, domain.Tenant)
	m.enterHelpView()
	columns, _, titles := m.helpColumns()
	for off := range 20 {
		for rows := 3; rows < 25; rows++ {
			for _, col := range columns {
				if off+rows >= len(col) {
					continue
				}
				window := m.helpWindowLines(off, rows)
				require.Len(t, window, rows)
				last := col[off+rows-1]
				if titles[last] {
					assert.NotContains(t, window[rows-1], last, "title %q alone at off=%d rows=%d", ansi.Strip(last), off, rows)
				}
			}
		}
	}
}

// The export folder list takes the theme's colors, not the filepicker's.
func TestPickerStyles_Themed(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	p := m.theme.Palette
	assert.Equal(t, p.Accent, m.dirPicker.Styles.Cursor.GetForeground())
	assert.Equal(t, p.Text, m.dirPicker.Styles.Directory.GetForeground())
	assert.Equal(t, p.Accent, m.dirPicker.Styles.Selected.GetForeground())
	assert.Equal(t, p.Muted, m.dirPicker.Styles.File.GetForeground())
	assert.Contains(t, m.dirPicker.Styles.EmptyDirectory.Render(), "no folders here", "keeps its text")
}

// The chrome has one fill — PROD — and keys are the only orange.
func TestChrome_OneFill(t *testing.T) {
	t.Parallel()
	s := DefaultStyles(true)
	assert.Equal(t, s.Palette.DangerFill, s.Prod.GetBackground())
	assert.Equal(t, s.Palette.Primary, s.HelpKey.GetForeground(), "help keys are the footer's orange")
	assert.NotContains(t, s.Context.Render("x"), "48;", "category is text, not a chip")
	assert.NotContains(t, s.Stats.Render("x"), "48;")
	assert.NotContains(t, s.Live.Render("x"), "48;")
	assert.NotContains(t, s.LogFollowing.Render("x"), "48;")
}
