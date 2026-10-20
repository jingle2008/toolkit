package models

// GPUWorkload is a GPU-consuming Kubernetes pod (any pod that requests
// nvidia.com/gpu or amd.com/gpu). Grouped by Node (spec.nodeName, which
// equals GPUNode.Name); its parent category is GPUNode. model/runtime/mode are
// best-effort: blank for non-serving GPU pods.
type GPUWorkload struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	TenantID  string `json:"tenantId"`
	Namespace string `json:"namespace"`
	Model     string `json:"model,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
	GPUs      int    `json:"gpus"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
	// Status is the pod phase: Running, or Pending for a pod scheduled to
	// its node (holding its GPUs) that hasn't started yet.
	Status string `json:"status"`
	Mode   string `json:"mode,omitempty"`
	// Legacy is true for a pod serving a legacy ome.oracle.com/v1alpha1
	// InferenceService; its Runtime then names a legacy runtime, not an
	// ome.io ClusterServingRuntime.
	Legacy bool    `json:"legacy,omitempty"`
	Owner  *Tenant `json:"owner,omitempty"`
}

// GetName returns the pod name.
func (w GPUWorkload) GetName() string { return w.Name }

// IsFaulty reports whether the workload's containers have restarted,
// which usually signals a crash-looping or otherwise unhealthy pod.
func (w GPUWorkload) IsFaulty() bool { return w.Restarts > 0 }

// FilterableFields returns the fields matched by `--filter`. Age is
// excluded (matching GPUNode): a formatted duration like "3d" is noise
// for text filtering.
func (w GPUWorkload) FilterableFields() []string {
	fields := []string{w.Name, w.Node, w.TenantID, w.Namespace, w.Model, w.Runtime, w.Mode}
	if w.Legacy {
		fields = append(fields, "legacy")
	}
	return fields
}

// TenancyOCID returns the full tenancy OCID from realm + tenancy-id suffix.
func (w GPUWorkload) TenancyOCID(realm string) string {
	return tenancyOCID(realm, w.TenantID)
}

// TenantName returns the resolved owning-tenant name, or the raw
// tenancy-id suffix when the owner is unresolved.
func (w GPUWorkload) TenantName() string {
	if w.Owner != nil {
		return w.Owner.Name
	}
	return w.TenantID
}
