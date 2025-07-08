package models

import "fmt"

// DedicatedAICluster represents a dedicated AI cluster resource.
type DedicatedAICluster struct {
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	TenantID      string  `json:"tenantId"`
	Type          string  `json:"type,omitempty"`
	UnitShape     string  `json:"unitShape,omitempty"`
	Size          int     `json:"size,omitempty"`
	Profile       string  `json:"profile,omitempty"`
	Owner         *Tenant `json:"owner,omitempty"`
	ModelName     string  `json:"modelName,omitempty"`
	TotalReplicas int     `json:"totalReplicas"`
	IdleReplicas  int     `json:"idleReplicas"`
}

// GetName returns the name of the dedicated AI cluster.
func (n DedicatedAICluster) GetName() string {
	return n.Name
}

// GetFilterableFields returns filterable fields for the dedicated AI cluster.
func (n DedicatedAICluster) GetFilterableFields() []string {
	return []string{
		n.Name,
		n.Type,
		n.UnitShape,
		n.Status,
		n.TenantID,
		n.Profile,
		n.GetOwnerState(),
		n.ModelName,
		n.GetUsage(),
	}
}

// GetKey returns the key of the dedicated AI cluster.
func (n DedicatedAICluster) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", n.Type, n.UnitShape, n.Name)
}

func (n DedicatedAICluster) GetOwnerState() string {
	var state string
	if n.Owner != nil {
		state = fmt.Sprint(n.Owner.IsInternal)
	}
	return state
}

func (n DedicatedAICluster) GetUsage() string {
	if n.TotalReplicas <= 0 {
		return ""
	}

	rate := 1.0 - float64(n.IdleReplicas)/float64(n.TotalReplicas)
	return fmt.Sprintf("%.0f%%", rate*100)
}
