package models

import (
	"strconv"
	"strings"
)

// GpuNode represents a GPU node.
type GpuNode struct {
	Name                 string   `json:"name"`
	InstanceType         string   `json:"instanceType"`
	NodePool             string   `json:"poolName"`
	CompartmentID        string   `json:"compartmentId"`
	ID                   string   `json:"id"`
	Allocatable          int      `json:"allocatable"`
	Allocated            int      `json:"allocated"`
	IsReady              bool     `json:"isReady"`
	IsSchedulingDisabled bool     `json:"isSchedulingDisabled"` // true if node is cordoned
	Age                  string   `json:"age"`
	Issues               []string `json:"issues"`
	status               string
}

// GetName returns the name of the GPU node.
func (n GpuNode) GetName() string {
	return n.Name
}

// GetFilterableFields returns filterable fields for the GPU node.
func (n GpuNode) GetFilterableFields() []string {
	return []string{n.Name, n.InstanceType, n.NodePool, n.GetStatus()}
}

// SetStatus sets the status of the GPU node.
func (n *GpuNode) SetStatus(status string) {
	n.status = status
}

// GetStatus returns the status of the GPU node.
func (n GpuNode) GetStatus() string {
	if n.status != "" {
		return n.status
	}

	parts := strings.Split(n.InstanceType, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	switch {
	case n.IsSchedulingDisabled:
		return "WARN: CORDONED"
	case n.Allocatable != count:
		return "ERROR: Missing GPUs"
	case !n.IsHealthy():
		return "ERROR: Unhealthy"
	case !n.IsReady:
		return "ERROR: Not ready"
	}
	return "OK"
}

/*
IsHealthy returns true if the GPU node has no issues.
*/
func (n GpuNode) IsHealthy() bool {
	return len(n.Issues) == 0
}

// IsFaulty returns true if the node is cordoned, missing GPUs, unhealthy, or not ready.
func (n GpuNode) IsFaulty() bool {
	return n.GetStatus() != "OK"
}
