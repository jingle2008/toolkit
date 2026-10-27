package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
	// Age is where the output writes the time since CreatedAt (MarshalJSON);
	// it is never stored, so it can't go stale. Code reads AgeOf(CreatedAt).
	Age string `json:"age"`
	// CreatedAt is when the resource was created; zero when unknown.
	CreatedAt time.Time `json:"createdAt,omitzero"`
}

// GetName returns the name of the dedicated AI cluster.
func (n DedicatedAICluster) GetName() string {
	return n.Name
}

// FilterableFields returns filterable fields for the dedicated AI cluster.
func (n DedicatedAICluster) FilterableFields() []string {
	return []string{
		n.Name,
		n.Type,
		n.UnitShape,
		n.Status,
		n.TenantID,
		n.Profile,
		n.OwnerState(),
		n.ModelName,
		n.Usage(),
		AgeOf(n.CreatedAt),
	}
}

/*
OwnerState returns the state of the owner (internal/external) for the dedicated AI cluster.
*/
func (n DedicatedAICluster) OwnerState() string {
	var state string
	if n.Owner != nil {
		state = fmt.Sprint(n.Owner.IsInternal)
	}
	return state
}

/*
Usage returns the usage percentage as a string for the dedicated AI cluster.
*/
func (n DedicatedAICluster) Usage() string {
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
		return r.Code()
	}
	return region
}

// OCID returns the full OCID for the DedicatedAICluster, built from
// realm + region + the stored Name suffix.
func (n DedicatedAICluster) OCID(realm, region string) string {
	region = normalizeRegion(region)
	return fmt.Sprintf("ocid1.generativeaidedicatedaicluster.%s.%s.%s", realm, region, n.Name)
}

// TenancyOCID returns the full tenancy OCID for the
// DedicatedAICluster, built from realm + the stored TenantID suffix.
func (n DedicatedAICluster) TenancyOCID(realm string) string {
	return tenancyOCID(realm, n.TenantID)
}

// MarshalJSON writes DedicatedAICluster with its age as of now, from CreatedAt.
func (d DedicatedAICluster) MarshalJSON() ([]byte, error) {
	type plain DedicatedAICluster
	p := plain(d)
	p.Age = AgeOf(d.CreatedAt)
	return json.Marshal(p)
}
