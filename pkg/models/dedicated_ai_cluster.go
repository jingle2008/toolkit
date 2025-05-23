package models

import "fmt"

type DedicatedAICluster struct {
	// Common fields
	Name     string `json:"name"`
	Status   string `json:"status"`
	TenantId string `json:"tenantId"`

	// v1 fields
	Type      string `json:"type,omitempty"`
	UnitShape string `json:"unitShape,omitempty"`
	Size      int    `json:"size,omitempty"`

	// v2 fields
	Profile string `json:"profile,omitempty"`
}

func (n DedicatedAICluster) GetName() string {
	return n.Name
}

func (n DedicatedAICluster) GetFilterableFields() []string {
	return []string{n.Name, n.Type, n.UnitShape, n.Status, n.TenantId}
}

func (n DedicatedAICluster) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", n.Type, n.UnitShape, n.Name)
}
