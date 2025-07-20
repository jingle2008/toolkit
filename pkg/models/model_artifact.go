package models

import "fmt"

// ModelArtifact represents a model artifact stored in object storage.
type ModelArtifact struct {
	Name            string `json:"name"`
	TensorRTVersion string `json:"tensorrt_version"`
	GpuCount        int    `json:"gpu_count"`
	GpuShape        string `json:"gpu_shape"`
	ModelName       string `json:"model_name"`
}

// GetName returns the name of the model artifact.
func (m ModelArtifact) GetName() string {
	return m.Name
}

// GetGpuConfig returns the GPU configuration string for the model artifact.
func (m ModelArtifact) GetGpuConfig() string {
	return fmt.Sprintf("%dx %s", m.GpuCount, m.GpuShape)
}

// GetFilterableFields returns filterable fields for the model artifact.
func (m ModelArtifact) GetFilterableFields() []string {
	return []string{m.Name, m.GetGpuConfig(), m.ModelName}
}

// IsFaulty returns false by default for ModelArtifact.
func (m ModelArtifact) IsFaulty() bool {
	return false
}
