package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGpuPool_Getters(t *testing.T) {
	pool := GpuPool{
		Name:         "pool1",
		Shape:        "NVIDIA.A100.8",
		CapacityType: "dedicated",
		Size:         1,
	}
	assert.Equal(t, "pool1", pool.GetName())
	assert.ElementsMatch(t, []string{"pool1", "NVIDIA.A100.8", "dedicated"}, pool.GetFilterableFields())
	assert.Equal(t, 8, pool.GetGPUs())

	pool2 := GpuPool{
		Name:         "pool2",
		Shape:        "NVIDIA.A100.8",
		CapacityType: "dedicated",
		Size:         2,
	}
	assert.Equal(t, 16, pool2.GetGPUs())
}
