package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestBlend(t *testing.T) {
	t.Parallel()
	a, b := lipgloss.Color("#FF8700"), lipgloss.Color("#FF5F87")
	assert.Equal(t, lipgloss.Color("#FF8700"), blend(a, b, 0))
	assert.Equal(t, lipgloss.Color("#FF5F87"), blend(a, b, 1))
	assert.Equal(t, lipgloss.Color("#FF7344"), blend(a, b, 0.5))
	assert.Equal(t, lipgloss.Color("208"), blend("208", b, 0.5), "non-hex falls back to a")
}

func TestRenderLogo_KeepsShape(t *testing.T) {
	t.Parallel()
	rows := strings.Split(ansi.Strip(RenderLogo("#FF8700", "#FF5F87")), "\n")
	want := strings.Split(Logo, "\n")
	assert.Len(t, rows, len(want))
	for i, row := range rows {
		assert.Equal(t, LogoWidth+LogoMargin, ansi.StringWidth(row), "row %d width", i)
		assert.Equal(t, want[i], strings.TrimRight(row, " "), "row %d", i)
	}
}
