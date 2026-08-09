package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPUPool_Getters(t *testing.T) {
	t.Parallel()
	pool := GPUPool{
		Name:         "pool1",
		Shape:        "NVIDIA.A100.8",
		CapacityType: "dedicated",
		Size:         1,
	}
	assert.Equal(t, "pool1", pool.GetName())
	assert.ElementsMatch(t, []string{"pool1", "NVIDIA.A100.8", "dedicated"}, pool.FilterableFields())
	assert.Equal(t, 8, pool.GPUs())

	pool2 := GPUPool{
		Name:         "pool2",
		Shape:        "NVIDIA.A100.8",
		CapacityType: "dedicated",
		Size:         2,
	}
	assert.Equal(t, 16, pool2.GPUs())
}

func TestGPUPool_IsFaulty(t *testing.T) {
	t.Parallel()
	// Faulty when the actual size has drifted from the desired size.
	assert.False(t, GPUPool{Size: 2, ActualSize: 2}.IsFaulty())
	assert.True(t, GPUPool{Size: 2, ActualSize: 1}.IsFaulty())
	assert.True(t, GPUPool{Size: 2, ActualSize: 3}.IsFaulty())
}
