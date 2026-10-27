package models

import (
	"encoding/json"
	"strconv"
	"time"
)

// Inference service types. Not a table column; carried in -o json and
// the MCP output, and matched by filtering.
const (
	InferenceServiceDedicated = "dedicated"
	InferenceServiceOnDemand  = "on-demand"
)

// InferenceService is an OME InferenceService CR — one serving endpoint.
// Grouped by Namespace, which for a dedicated endpoint is the DAC ID
// (== DedicatedAICluster.Name); its parent category is
// DedicatedAICluster. On-demand endpoints live in a namespace named
// after their model and carry no tenant.
type InferenceService struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	TenantID  string `json:"tenantId,omitempty"`
	Model     string `json:"model,omitempty"`
	// ImportedModel is true when the endpoint serves a tenant-imported
	// model rather than a shared-catalog one: a namespaced BaseModel ref,
	// or a ClusterBaseModel ref whose name is an imported model's OCID (a
	// tenant-labelled ClusterBaseModel).
	ImportedModel bool   `json:"importedModel,omitempty"`
	Runtime       string `json:"runtime,omitempty"`
	// MinReplicas and MaxReplicas are the engine's replica range; nil
	// when the spec doesn't set one (an endpoint with an empty spec).
	MinReplicas *int   `json:"minReplicas,omitempty"`
	MaxReplicas *int   `json:"maxReplicas,omitempty"`
	Ready       string `json:"ready"`
	// Age is where the output writes the time since CreatedAt (MarshalJSON);
	// it is never stored, so it can't go stale. Code reads AgeOf(CreatedAt).
	Age string `json:"age"`
	// CreatedAt is when the resource was created; zero when unknown.
	CreatedAt time.Time `json:"createdAt,omitzero"`
	// Legacy is true for an ome.oracle.com/v1alpha1 InferenceService —
	// an endpoint on a legacy DAC.
	Legacy bool    `json:"legacy,omitempty"`
	Owner  *Tenant `json:"owner,omitempty"`
}

// GetName returns the inference service name.
func (s InferenceService) GetName() string { return s.Name }

// IsFaulty reports whether the Ready condition is anything but True.
func (s InferenceService) IsFaulty() bool { return s.Ready != "True" }

// FilterableFields returns the fields matched by `--filter`. Age is
// excluded (matching GPUWorkload): a formatted duration is noise for
// text filtering.
// TenantName is matched alongside TenantID so the Tenant column filters
// by what it shows.
func (s InferenceService) FilterableFields() []string {
	fields := []string{s.Name, s.Namespace, s.Type, s.TenantID, s.TenantName(), s.Model, s.Runtime, s.Ready}
	if s.Legacy {
		fields = append(fields, "legacy")
	}
	return fields
}

// IsDedicated reports whether the endpoint runs on a dedicated AI cluster.
func (s InferenceService) IsDedicated() bool { return s.Type == InferenceServiceDedicated }

// TenancyOCID returns the full tenancy OCID from realm + tenancy-id
// suffix, or "" for an endpoint without a tenant.
func (s InferenceService) TenancyOCID(realm string) string {
	return tenancyOCID(realm, s.TenantID)
}

// TenantName returns the resolved owning-tenant name, or the raw
// tenancy-id suffix when the owner is unresolved.
func (s InferenceService) TenantName() string {
	if s.Owner != nil {
		return s.Owner.Name
	}
	return s.TenantID
}

// Replicas renders the engine replica range as "min/max", "" when the
// spec sets neither (drawn as —, not a misleading 0/0), and — for the
// one it doesn't set.
func (s InferenceService) Replicas() string {
	if s.MinReplicas == nil && s.MaxReplicas == nil {
		return ""
	}
	part := func(n *int) string {
		if n == nil {
			return "—"
		}
		return strconv.Itoa(*n)
	}
	return part(s.MinReplicas) + "/" + part(s.MaxReplicas)
}

// MarshalJSON writes InferenceService with its age as of now, from CreatedAt.
func (s InferenceService) MarshalJSON() ([]byte, error) {
	type plain InferenceService
	p := plain(s)
	p.Age = AgeOf(s.CreatedAt)
	return json.Marshal(p)
}
