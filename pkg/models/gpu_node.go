package models

import (
	"strconv"
	"strings"
)

// GpuNode represents a GPU node.
type GpuNode struct {
	Name         string `json:"name"`
	InstanceType string `json:"instanceType"`
	NodePool     string `json:"poolName"`
	Allocatable  int    `json:"allocatable"`
	Allocated    int    `json:"allocated"`
	IsHealthy    bool   `json:"isHealthy"`
	IsReady      bool   `json:"isReady"`
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
	if n.Allocatable != count {
		return "ERROR: Missing GPUs"
	} else if !n.IsHealthy {
		return "ERROR: Unhealthy"
	} else if !n.IsReady {
		return "ERROR: Not ready"
	}

	return "OK"
}
