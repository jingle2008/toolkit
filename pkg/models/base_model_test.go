package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseModelFields(t *testing.T) {
	bm := BaseModel{
		Capabilities: []string{"generation", "embedding"},
		DacShapeConfigs: &DacShapeConfigs{
			CompatibleDACShapes: []DACShape{
				{Name: "SMALL", QuotaUnit: 2, Default: true},
			},
		},
	}
	assert.Equal(t, []string{"embedding", "generation"}, bm.Capabilities)
	assert.Equal(t, "SMALL", bm.DacShapeConfigs.CompatibleDACShapes[0].Name)
}

func TestBaseModelIsFaulty(t *testing.T) {
	bm := BaseModel{}
	assert.False(t, bm.IsFaulty())
}
