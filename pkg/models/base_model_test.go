package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseModel_Getters(t *testing.T) {
	t.Parallel()
	// IsFaulty always returns false
	assert.False(t, BaseModel{}.IsFaulty())

	// Panic on duplicate default DAC shape
	t.Run("panic on duplicate default DAC", func(t *testing.T) {
		cap1 := &Capability{
			ChartValues: &ChartValues{
				ModelMetaData: &ModelMetaData{
					DacShapeConfigs: &DacShapeConfigs{
						CompatibleDACShapes: []DACShape{
							{Name: "A", Default: true},
						},
					},
				},
			},
		}
		cap2 := &Capability{
			ChartValues: &ChartValues{
				ModelMetaData: &ModelMetaData{
					DacShapeConfigs: &DacShapeConfigs{
						CompatibleDACShapes: []DACShape{
							{Name: "B", Default: true},
						},
					},
				},
			},
		}
		bm := BaseModel{
			InternalName: "panic-test",
			Capabilities: map[string]*Capability{
				"cap1": cap1,
				"cap2": cap2,
			},
		}
		assert.Panics(t, func() { bm.GetDefaultDacShape() })
	})

	bm := BaseModel{
		Type:     "testType",
		Name:     "testName",
		Version:  "v1",
		Category: "catA",
		Vendor:   "vendorA",
		Capabilities: map[string]*Capability{
			"cap1": {Capability: "cap1", Replicas: 0},
			"cap2": {Capability: "cap2", Replicas: 2},
		},
		IsExperimental:      true,
		IsInternal:          true,
		IsLongTermSupported: true,
		LifeCyclePhase:      "DEPRECATED",
	}
	assert.Equal(t, "testName", bm.GetName())
	assert.ElementsMatch(t, []string{"C", "C*2"}, bm.GetCapabilities())
	// GetDefaultDacShape returns nil unless Capabilities have ChartValues with DacShapeConfigs, so just check type
	assert.Nil(t, bm.GetDefaultDacShape())
	fields := bm.GetFilterableFields()
	assert.Contains(t, fields, "testName")
	assert.Contains(t, fields, "testType")
	assert.Contains(t, fields, "catA")
	assert.Contains(t, fields, "vendorA")
	assert.Contains(t, fields, "v1")
	assert.Contains(t, fields, "EXP/INT/LTS/RTD")
}
