package view

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestScrollWindow(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 2, ScrollWindow(2, 5, 3), "cursor above the window pulls it up")
	assert.Equal(t, 3, ScrollWindow(5, 0, 3), "cursor below the window pushes it down")
	assert.Equal(t, 1, ScrollWindow(2, 1, 3), "cursor inside leaves it")
}

func TestFixedModal_KeepsSize(t *testing.T) {
	t.Parallel()
	small := FixedModal("T", "a", "hint", color.White, color.Black, 0, 20, 5)
	big := FixedModal("T", strings.Repeat("x", 20)+"\n2\n3\n4\n5", "hint", color.White, color.Black, 0, 20, 5)
	assert.Equal(t, lipgloss.Height(big), lipgloss.Height(small))
	assert.Equal(t, lipgloss.Width(big), lipgloss.Width(small))
}
