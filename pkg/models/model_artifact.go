package models

import "fmt"

// ModelArtifact represents a model artifact stored in object storage.
type ModelArtifact struct {
	Name            string `json:"name"`
	TensorRTVersion string `json:"tensorrt_version"`
	GPUCount        int    `json:"gpu_count"`
	GPUShape        string `json:"gpu_shape"`
	ModelName       string `json:"model_name"`
}

// GetName returns the name of the model artifact.
func (m ModelArtifact) GetName() string {
	return m.Name
}

// GPUConfig returns the GPU configuration string for the model artifact.
func (m ModelArtifact) GPUConfig() string {
	return fmt.Sprintf("%dx %s", m.GPUCount, m.GPUShape)
}

// FilterableFields returns filterable fields for the model artifact.
func (m ModelArtifact) FilterableFields() []string {
	return []string{m.Name, m.GPUConfig(), m.ModelName}
}

// IsFaulty returns false by default for ModelArtifact.
func (m ModelArtifact) IsFaulty() bool {
	return false
}
