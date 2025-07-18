package models

import (
	"strconv"
	"strings"
)

// GpuNode represents a GPU node.
type GpuNode struct {
	Name                 string `json:"name"`
	InstanceType         string `json:"instanceType"`
	NodePool             string `json:"poolName"`
	Allocatable          int    `json:"allocatable"`
	Allocated            int    `json:"allocated"`
	IsHealthy            bool   `json:"isHealthy"`
	IsReady              bool   `json:"isReady"`
	IsSchedulingDisabled bool   `json:"isSchedulingDisabled"` // true if node is cordoned
	Age                  string `json:"age"`
}

// GetName returns the name of the GPU node.
func (n GpuNode) GetName() string {
	return n.Name
}

// GetFilterableFields returns filterable fields for the GPU node.
func (n GpuNode) GetFilterableFields() []string {
	return []string{n.Name, n.InstanceType, n.NodePool, n.GetStatus()}
}

// GetStatus returns the status of the GPU node.
func (n GpuNode) GetStatus() string {
	parts := strings.Split(n.InstanceType, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	switch {
	case n.IsSchedulingDisabled:
		return "WARN: CORDONED"
	case n.Allocatable != count:
		return "ERROR: Missing GPUs"
	case !n.IsHealthy:
		return "ERROR: Unhealthy"
	case !n.IsReady:
		return "ERROR: Not ready"
	}
	return "OK"
}

// IsFaulty returns true if the node is cordoned, missing GPUs, unhealthy, or not ready.
func (n GpuNode) IsFaulty() bool {
	return n.GetStatus() != "OK"
}
