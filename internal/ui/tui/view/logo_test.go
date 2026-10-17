package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestBlend(t *testing.T) {
	t.Parallel()
	a, b := RGB{0xFF, 0x87, 0x00}, RGB{0xFF, 0x5F, 0x87}
	assert.Equal(t, a, blend(a, b, 0))
	assert.Equal(t, b, blend(a, b, 1))
	assert.Equal(t, "#FF7344", blend(a, b, 0.5).Hex())
}

func TestRenderLogo_KeepsShape(t *testing.T) {
	t.Parallel()
	rows := strings.Split(ansi.Strip(RenderLogo(RGB{0xFF, 0x87, 0x00}, RGB{0xFF, 0x5F, 0x87})), "\n")
	want := strings.Split(Logo, "\n")
	assert.Len(t, rows, len(want))
	for i, row := range rows {
		assert.Equal(t, LogoWidth+LogoMargin, ansi.StringWidth(row), "row %d width", i)
		assert.Equal(t, want[i], strings.TrimRight(row, " "), "row %d", i)
	}
}
