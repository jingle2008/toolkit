package toolkit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCenterText_EvenWidth(t *testing.T) {
	text := "foo"
	out := centerText(text, 10, 1)
	assert.Equal(t, "   foo    ", out)
}

func TestCenterText_OddWidth(t *testing.T) {
	text := "bar"
	out := centerText(text, 9, 1)
	assert.Equal(t, "   bar   ", out)
}

func TestCenterText_WidthLessThanText(t *testing.T) {
	text := "longtext"
	out := centerText(text, 4, 1)
	assert.Equal(t, "long\ntext", out)
}

func TestCenterText_Height(t *testing.T) {
	text := "baz"
	out := centerText(text, 7, 3)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, "  baz  ", lines[1])
}

func TestCenterText_EmptyText(t *testing.T) {
	out := centerText("", 5, 2)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, "     ", lines[0])
	assert.Equal(t, "     ", lines[1])
}
