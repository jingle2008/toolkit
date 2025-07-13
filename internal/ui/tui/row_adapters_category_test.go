package tui

import (
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCategoryRow_RenderAndToRow(t *testing.T) {
	t.Parallel()
	cat := domain.GpuNode
	row := CategoryRow(cat)
	rendered := row.Render("")
	assert.Len(t, rendered, 2)
	assert.Equal(t, "GpuNode", rendered[0])
	aliases := strings.Split(rendered[1], ", ")
	assert.Contains(t, aliases, "gn")
	assert.Contains(t, aliases, "gpunode")

	toRow := row.ToRow("ignored")
	assert.Equal(t, rendered, []string(toRow))
}
