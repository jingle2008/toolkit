package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPUNode_Getters(t *testing.T) {
	t.Parallel()
	node := GPUNode{
		Name:         "node1",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolA",
		Allocatable:  8,
		Allocated:    4,
		IsReady:      true,
	}
	assert.Equal(t, "node1", node.GetName())
	assert.ElementsMatch(t, []string{"node1", "NVIDIA.A100.8", "poolA", "OK"}, node.FilterableFields())
	assert.Equal(t, "OK", node.GetStatus())

	node2 := GPUNode{
		Name:         "node2",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolB",
		Allocatable:  7, // mismatch
		IsReady:      true,
	}
	assert.Equal(t, "ERROR: Missing GPUs", node2.GetStatus())

	node3 := GPUNode{
		Name:         "node3",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolC",
		Allocatable:  8,
		Issues:       []string{"bad"},
		IsReady:      true,
	}
	assert.Equal(t, "ERROR: Unhealthy", node3.GetStatus())

	node4 := GPUNode{
		Name:         "node4",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolD",
		Allocatable:  8,
		IsReady:      false,
	}
	assert.Equal(t, "ERROR: Not ready", node4.GetStatus())
}

func TestGPUNode_GetStatus_Cordoned(t *testing.T) {
	t.Parallel()
	// Cordoning wins over every other condition.
	node := GPUNode{
		InstanceType:         "NVIDIA.A100.8",
		Allocatable:          7,
		IsSchedulingDisabled: true,
		Issues:               []string{"bad"},
	}
	assert.Equal(t, "WARN: CORDONED", node.GetStatus())
}

func TestGPUNode_IsHealthy(t *testing.T) {
	t.Parallel()
	assert.True(t, GPUNode{}.IsHealthy())
	assert.False(t, GPUNode{Issues: []string{"bad"}}.IsHealthy())
}

func TestGPUNode_IsFaulty(t *testing.T) {
	t.Parallel()
	healthy := GPUNode{InstanceType: "NVIDIA.A100.8", Allocatable: 8, IsReady: true}
	assert.False(t, healthy.IsFaulty())

	notReady := GPUNode{InstanceType: "NVIDIA.A100.8", Allocatable: 8, IsReady: false}
	assert.True(t, notReady.IsFaulty())
}
