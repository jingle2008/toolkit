package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// sectionsOfLen builds sections with the given line counts (title included).
func sectionsOfLen(lens ...int) []helpSection {
	out := make([]helpSection, len(lens))
	for i, n := range lens {
		out[i] = make(helpSection, n)
	}
	return out
}

// packColumns picks the split with the shortest tallest column. GPUNode's
// help with three quick-switch slots (4, 8, 5, 12, 7, 9 lines) used to fill
// a column to an even share and pile the rest into the last: 13, 5, 30.
func TestPackColumns_Balanced(t *testing.T) {
	t.Parallel()
	cols := packColumns(sectionsOfLen(4, 8, 5, 12, 7, 9), 3)
	heights := make([]int, len(cols))
	for i, c := range cols {
		heights[i] = len(c)
	}
	assert.Equal(t, []int{13, 18, 17}, heights)

	// Ties go to fewer columns: [4] [1 1] is as tall as [4] [1] [1].
	cols = packColumns(sectionsOfLen(4, 1, 1), 3)
	require.Len(t, cols, 2)
	assert.Len(t, cols[1], 3, "two sections and the blank between them")
	assert.Len(t, packColumns(nil, 3), 1)
}

// Faulty-only and refresh act on the list, so help files them with the
// view's keys rather than under Actions.
func TestHelpSections_ListLevelKeysWithView(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 200, 60)
	showCategory(t, m, domain.GPUNode)
	m.Update(keyPress("?"))
	sections := map[string]string{}
	for _, s := range m.helpSections() {
		sections[strings.TrimSpace(ansi.Strip(s[0]))] = ansi.Strip(strings.Join(s[1:], "\n"))
	}
	require.Contains(t, sections, "Actions")
	assert.NotContains(t, sections["Actions"], "Faulty only")
	assert.NotContains(t, sections["Actions"], "Refresh")
	assert.Contains(t, sections["Actions"], "Terminate")
	assert.Contains(t, sections["List view"], "Faulty only")
	assert.Contains(t, sections["List view"], "Refresh")
}
