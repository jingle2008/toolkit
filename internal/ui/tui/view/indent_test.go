package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndentLines(t *testing.T) {
	t.Parallel()
	assert.Equal(t, " a\n b", IndentLines("a\nb", " "))
	assert.Equal(t, "  ", IndentLines("", "  "))
}
