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
	// Faulty when short of the desired size; past it is a deliberate
	// over-scale, not a fault.
	assert.False(t, GPUPool{Size: 2, ActualSize: 2}.IsFaulty())
	assert.True(t, GPUPool{Size: 2, ActualSize: 1}.IsFaulty())
	assert.False(t, GPUPool{Size: 2, ActualSize: 3}.IsFaulty())
	// An actual size not looked up yet, or that couldn't be, is unknown,
	// not zero; a pool OCI lacks is really empty.
	assert.False(t, GPUPool{Size: 2, Status: GPUPoolStatusPending}.IsFaulty())
	assert.False(t, GPUPool{Size: 2, Status: GPUPoolStatusUnknown}.IsFaulty())
	assert.True(t, GPUPool{Size: 2, Status: GPUPoolStatusNonexistent}.IsFaulty())
}

func TestGPUPool_IsOverSize(t *testing.T) {
	t.Parallel()
	assert.True(t, GPUPool{Size: 2, ActualSize: 3}.IsOverSize())
	assert.False(t, GPUPool{Size: 2, ActualSize: 2}.IsOverSize())
	assert.False(t, GPUPool{Size: 2, ActualSize: 3, Status: GPUPoolStatusUnknown}.IsOverSize(), "a stale actual size isn't drift")
}

// The default count fills a short pool, and goes one past a full one.
func TestGPUPool_DefaultScaleAdd(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 3, GPUPool{Size: 4, ActualSize: 1}.DefaultScaleAdd())
	assert.Equal(t, 1, GPUPool{Size: 4, ActualSize: 4}.DefaultScaleAdd())
	assert.Equal(t, 1, GPUPool{Size: 4, ActualSize: 6}.DefaultScaleAdd())
	assert.Equal(t, 0, GPUPool{Size: 4, ActualSize: 6}.Shortfall())
}
