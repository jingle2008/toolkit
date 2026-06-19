package models

import "fmt"

// GPUWorkload is a GPU-consuming Kubernetes pod (any pod that requests
// nvidia.com/gpu). Grouped by Node (spec.nodeName, which equals
// GPUNode.Name); its parent category is GPUNode. model/runtime/mode are
// best-effort: blank for non-serving GPU pods.
type GPUWorkload struct {
	Name      string  `json:"name"`
	Node      string  `json:"node"`
	TenantID  string  `json:"tenantId"`
	Namespace string  `json:"namespace"`
	Model     string  `json:"model,omitempty"`
	Runtime   string  `json:"runtime,omitempty"`
	GPUs      int     `json:"gpus"`
	Restarts  int     `json:"restarts"`
	Age       string  `json:"age"`
	Mode      string  `json:"mode,omitempty"`
	Owner     *Tenant `json:"owner,omitempty"`
}

// GetName returns the pod name.
func (w GPUWorkload) GetName() string { return w.Name }

// IsFaulty reports whether the workload's containers have restarted,
// which usually signals a crash-looping or otherwise unhealthy pod.
func (w GPUWorkload) IsFaulty() bool { return w.Restarts > 0 }

// FilterableFields returns the fields matched by `--filter`.
func (w GPUWorkload) FilterableFields() []string {
	return []string{w.Name, w.Node, w.TenantID, w.Namespace, w.Model, w.Runtime, w.Mode, w.Age}
}

// TenancyOCID returns the full tenancy OCID from realm + tenancy-id suffix.
func (w GPUWorkload) TenancyOCID(realm string) string {
	return fmt.Sprintf("ocid1.tenancy.%s..%s", realm, w.TenantID)
}

// TenantName returns the resolved owning-tenant name, or the raw
// tenancy-id suffix when the owner is unresolved.
func (w GPUWorkload) TenantName() string {
	if w.Owner != nil {
		return w.Owner.Name
	}
	return w.TenantID
}
