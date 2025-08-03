package models

import (
	"fmt"
	"strings"
)

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
	Age           string  `json:"age"`
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
		n.Age,
	}
}

/*
GetOwnerState returns the state of the owner (internal/external) for the dedicated AI cluster.
*/
func (n DedicatedAICluster) GetOwnerState() string {
	var state string
	if n.Owner != nil {
		state = fmt.Sprint(n.Owner.IsInternal)
	}
	return state
}

/*
GetUsage returns the usage percentage as a string for the dedicated AI cluster.
*/
func (n DedicatedAICluster) GetUsage() string {
	if n.TotalReplicas <= 0 {
		return ""
	}

	rate := 1.0 - float64(n.IdleReplicas)/float64(n.TotalReplicas)
	return fmt.Sprintf("%.0f%%", rate*100)
}

// IsFaulty returns true if the cluster status is "fail" or "failed".
func (n DedicatedAICluster) IsFaulty() bool {
	switch s := n.Status; {
	case len(s) == 0:
		return false
	default:
		lower := strings.ToLower(s)
		return lower == "fail" || lower == "failed"
	}
}

func normalizeRegion(region string) string {
	r := Region(region)
	if r == RegionIAD || r == RegionPHX {
		return r.GetCode()
	}
	return region
}

// GetID returns the OCID for the DedicatedAICluster.
func (n DedicatedAICluster) GetID(realm, region string) string {
	region = normalizeRegion(region)
	return fmt.Sprintf("ocid1.generativeaidedicatedaicluster.%s.%s.%s", realm, region, n.Name)
}

// GetTenantID returns the OCID for the tenancy of the DedicatedAICluster.
func (n DedicatedAICluster) GetTenantID(realm string) string {
	return fmt.Sprintf("ocid1.tenancy.%s..%s", realm, n.TenantID)
}
