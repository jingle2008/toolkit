package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// headerLines returns the frame's header rows (the info block's height),
// ANSI-stripped.
func headerLines(m *Model) []string {
	rows := lipgloss.Height(m.headerInfo())
	lines := strings.Split(ansi.Strip(m.frame("")), "\n")
	return lines[:rows]
}

func TestFrame_LogoFlushRight(t *testing.T) {
	t.Parallel()
	const w = 160
	m := newTestModel(t)
	m.updateLayout(w, 40)
	logo := strings.Split(view.Logo, "\n")
	for i, line := range headerLines(m) {
		assert.Equal(t, w, ansi.StringWidth(line), "row %d width", i)
		if i < len(logo) {
			want := logo[i] + strings.Repeat(" ", view.LogoWidth-ansi.StringWidth(logo[i])+view.LogoMargin)
			assert.True(t, strings.HasSuffix(line, want), "row %d = %q, want suffix %q", i, line, want)
		}
	}
}

func TestFrame_LogoDroppedWhenInfoAloneFits(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	w := lipgloss.Width(m.headerInfo()) + view.LogoWidth + view.LogoMargin - 1
	m.updateLayout(w, 40)
	require.False(t, m.showLogo(w))
	assert.NotContains(t, ansi.Strip(m.frame("")), "|_   _|")
}

func TestFrame_HeaderNeverExceedsTerminal(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// A wide category, so help actually presses against its budget.
	m.keys = keys.ResolveKeys(domain.GPUNode, common.ListView)
	for w := 30; w <= 160; w++ {
		m.updateLayout(w, 40)
		for i, line := range headerLines(m) {
			assert.LessOrEqual(t, ansi.StringWidth(line), w, "width %d row %d: %q", w, i, line)
		}
	}
}
