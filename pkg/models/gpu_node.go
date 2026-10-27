package models

import (
	"strconv"
	"strings"
	"time"
)

// GPUNode represents a GPU node.
type GPUNode struct {
	Name                 string `json:"name"`
	InstanceType         string `json:"instanceType"`
	NodePool             string `json:"poolName"`
	CompartmentID        string `json:"compartmentId"`
	ID                   string `json:"id"`
	Allocatable          int    `json:"allocatable"`
	Allocated            int    `json:"allocated"`
	IsReady              bool   `json:"isReady"`
	IsSchedulingDisabled bool   `json:"isSchedulingDisabled"` // true if node is cordoned
	Age                  string `json:"age"`
	// CreatedAt is when the resource was created; Age is the time since,
	// as of the load. Zero when unknown.
	CreatedAt time.Time `json:"createdAt,omitzero"`
	Issues    []string  `json:"issues"`
}

// GetName returns the name of the GPU node.
func (n GPUNode) GetName() string {
	return n.Name
}

// FilterableFields returns filterable fields for the GPU node.
func (n GPUNode) FilterableFields() []string {
	return []string{n.Name, n.InstanceType, n.NodePool, n.GetStatus()}
}

// GetStatus returns the status of the GPU node, derived from its state.
// (An action running on the node shows over it in the TUI's table, from
// the action tracker.)
func (n GPUNode) GetStatus() string {
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
func (n GPUNode) IsHealthy() bool {
	return len(n.Issues) == 0
}

// IsFaulty returns true if the node is cordoned, missing GPUs, unhealthy, or not ready.
func (n GPUNode) IsFaulty() bool {
	return n.GetStatus() != "OK"
}
