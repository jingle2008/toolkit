package models

import "fmt"

// DedicatedAICluster represents a dedicated AI cluster resource.
type DedicatedAICluster struct {
	// Common fields
	Name     string `json:"name"`
	Status   string `json:"status"`
	TenantID string `json:"tenantId"`

	// v1 fields
	Type      string `json:"type,omitempty"`
	UnitShape string `json:"unitShape,omitempty"`
	Size      int    `json:"size,omitempty"`

	// v2 fields
	Profile string `json:"profile,omitempty"`
}

// GetName returns the name of the dedicated AI cluster.
func (n DedicatedAICluster) GetName() string {
	return n.Name
}

// GetFilterableFields returns filterable fields for the dedicated AI cluster.
func (n DedicatedAICluster) GetFilterableFields() []string {
	return []string{n.Name, n.Type, n.UnitShape, n.Status, n.TenantID}
}

// GetKey returns the key of the dedicated AI cluster.
func (n DedicatedAICluster) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", n.Type, n.UnitShape, n.Name)
}
