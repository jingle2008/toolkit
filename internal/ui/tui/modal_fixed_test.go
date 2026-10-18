package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// modalBox returns the modal's top row, left column and width: the first
// ╭─ below the frame's own top border.
func modalBox(t *testing.T, screen string) [3]int {
	t.Helper()
	for i, l := range strings.Split(ansi.Strip(screen), "\n") {
		r := []rune(l)
		for j := 1; i > 2 && j+1 < len(r); j++ {
			if r[j] == '╭' && r[j+1] == '─' {
				for k := j; k < len(r); k++ {
					if r[k] == '╮' {
						return [3]int{i, j, k - j + 1}
					}
				}
			}
		}
	}
	require.Fail(t, "no modal on screen")
	return [3]int{}
}

// A filterable modal keeps its size and place as a filter is typed, down to
// no matches: the line being typed into doesn't move.
func TestFilterableModals_KeepSizeWhileFiltering(t *testing.T) {
	t.Parallel()
	targets := make([]domain.Scope, 20)
	for i := range targets {
		targets[i] = domain.Scope{Category: domain.GPUWorkload, Name: fmt.Sprintf("pod-%02d-%s", i, strings.Repeat("x", i*2))}
	}
	for name, tc := range map[string]struct {
		open  func(*Model)
		typed string
	}{
		"palette":    {func(m *Model) { m.Update(keyPress(":")) }, "gpunx"},
		"row picker": {func(m *Model) { m.openRowPicker("Go to", "go", targets, func(domain.Scope) tea.Cmd { return nil }) }, "1x9"},
		"help":       {func(m *Model) { m.Update(keyPress("?")) }, "/sorx"},
	} {
		m := newGoldenModel(t, 120, 40)
		showCategory(t, m, domain.DedicatedAICluster)
		tc.open(m)
		want := modalBox(t, m.render())
		for i, ch := range tc.typed {
			m.Update(keyPress(string(ch)))
			assert.Equalf(t, want, modalBox(t, m.render()), "%s after %q (top, left, width)", name, tc.typed[:i+1])
		}
	}
}
