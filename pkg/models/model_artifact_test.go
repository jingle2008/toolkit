package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelArtifact_Getters(t *testing.T) {
	t.Parallel()
	ma := ModelArtifact{
		ModelName: "m1",
		GpuShape:  "A100",
		GpuCount:  4,
		Name:      "artifact1",
	}
	assert.Equal(t, "artifact1", ma.GetName())
	assert.Equal(t, "4x A100", ma.GetGpuConfig())
	assert.ElementsMatch(t, []string{"artifact1", "4x A100", "m1"}, ma.GetFilterableFields())
}
