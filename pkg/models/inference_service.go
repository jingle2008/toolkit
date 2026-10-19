package models

import "fmt"

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
	MinReplicas   int    `json:"minReplicas"`
	MaxReplicas   int    `json:"maxReplicas"`
	Ready         string `json:"ready"`
	Age           string `json:"age"`
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
func (s InferenceService) FilterableFields() []string {
	fields := []string{s.Name, s.Namespace, s.Type, s.TenantID, s.Model, s.Runtime, s.Ready}
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

// Replicas renders the engine replica range as "min/max".
func (s InferenceService) Replicas() string {
	return fmt.Sprintf("%d/%d", s.MinReplicas, s.MaxReplicas)
}
