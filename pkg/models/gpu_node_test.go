package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGpuNode_Getters(t *testing.T) {
	t.Parallel()
	node := GpuNode{
		Name:         "node1",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolA",
		Allocatable:  8,
		Allocated:    4,
		IsReady:      true,
	}
	assert.Equal(t, "node1", node.GetName())
	assert.ElementsMatch(t, []string{"node1", "NVIDIA.A100.8", "poolA", "OK"}, node.GetFilterableFields())
	assert.Equal(t, "OK", node.GetStatus())

	node2 := GpuNode{
		Name:         "node2",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolB",
		Allocatable:  7, // mismatch
		IsReady:      true,
	}
	assert.Equal(t, "ERROR: Missing GPUs", node2.GetStatus())

	node3 := GpuNode{
		Name:         "node3",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolC",
		Allocatable:  8,
		Issues:       []string{"bad"},
		IsReady:      true,
	}
	assert.Equal(t, "ERROR: Unhealthy", node3.GetStatus())

	node4 := GpuNode{
		Name:         "node4",
		InstanceType: "NVIDIA.A100.8",
		NodePool:     "poolD",
		Allocatable:  8,
		IsReady:      false,
	}
	assert.Equal(t, "ERROR: Not ready", node4.GetStatus())
}
