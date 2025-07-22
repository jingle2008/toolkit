package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseModelFields(t *testing.T) {
	t.Parallel()
	bm := BaseModel{
		Capabilities: []string{"generation", "embedding"},
		DacShapeConfigs: &DacShapeConfigs{
			CompatibleDACShapes: []DACShape{
				{Name: "SMALL", QuotaUnit: 2, Default: true},
			},
		},
	}
	assert.ElementsMatch(t, []string{"embedding", "generation"}, bm.Capabilities)
	assert.Equal(t, "SMALL", bm.DacShapeConfigs.CompatibleDACShapes[0].Name)
}

func TestBaseModelIsFaulty(t *testing.T) {
	t.Parallel()
	bm := BaseModel{}
	assert.True(t, bm.IsFaulty())
}

func TestBaseModel_GetDefaultDacShape(t *testing.T) {
	t.Parallel()
	// No DacShapeConfigs
	bm := BaseModel{}
	assert.Nil(t, bm.GetDefaultDacShape())

	// DacShapeConfigs with no default
	bm = BaseModel{
		DacShapeConfigs: &DacShapeConfigs{
			CompatibleDACShapes: []DACShape{
				{Name: "A", QuotaUnit: 1, Default: false},
			},
		},
	}
	assert.Nil(t, bm.GetDefaultDacShape())

	// DacShapeConfigs with one default
	shape := DACShape{Name: "B", QuotaUnit: 2, Default: true}
	bm = BaseModel{
		DacShapeConfigs: &DacShapeConfigs{
			CompatibleDACShapes: []DACShape{
				{Name: "A", QuotaUnit: 1, Default: false},
				shape,
			},
		},
	}
	got := bm.GetDefaultDacShape()
	assert.NotNil(t, got)
	assert.Equal(t, "B", got.Name)
	assert.Equal(t, 2, got.QuotaUnit)
	assert.True(t, got.Default)
}

func TestBaseModel_GetFlags(t *testing.T) {
	t.Parallel()
	// All flags
	bm := BaseModel{
		IsExperimental:    true,
		IsInternal:        true,
		LifeCyclePhase:    "DEPRECATED",
		IsImageTextToText: true,
	}
	flags := bm.GetFlags()
	assert.Contains(t, flags, "EXP")
	assert.Contains(t, flags, "INT")
	assert.Contains(t, flags, "RTD")
	assert.Contains(t, flags, "IMG")

	// Only DAC
	bm = BaseModel{LifeCyclePhase: "ONDEMAND_SERVING_DISABLED"}
	assert.Equal(t, "DAC", bm.GetFlags())

	// No flags
	bm = BaseModel{}
	assert.Equal(t, "", bm.GetFlags())
}

func TestBaseModel_IsFaulty(t *testing.T) {
	t.Parallel()
	bm := BaseModel{Status: "Ready"}
	assert.False(t, bm.IsFaulty())
	bm = BaseModel{Status: "Creating"}
	assert.True(t, bm.IsFaulty())
	bm = BaseModel{Status: ""}
	assert.True(t, bm.IsFaulty())
}

func TestBaseModel_GetFilterableFields(t *testing.T) {
	t.Parallel()
	bm := BaseModel{
		Capabilities: []string{"capA", "capB"},
		Name:         "n",
		DisplayName:  "d",
		Status:       "s",
		Type:         "t",
		Version:      "v",
		Runtime:      "r",
	}
	fields := bm.GetFilterableFields()
	assert.Contains(t, fields, "capA")
	assert.Contains(t, fields, "capB")
	assert.Contains(t, fields, "n")
	assert.Contains(t, fields, "d")
	assert.Contains(t, fields, "s")
	assert.Contains(t, fields, "t")
	assert.Contains(t, fields, "r")
}

func TestDacShapeConfigs_Empty(t *testing.T) {
	t.Parallel()
	cfg := DacShapeConfigs{}
	assert.Empty(t, cfg.CompatibleDACShapes)
}
