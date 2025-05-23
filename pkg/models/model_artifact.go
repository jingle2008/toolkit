package models

import "fmt"

type ModelArtifact struct {
	Name            string `json:"name"`
	TensorRTVersion string `json:"tensorrt_version"`
	GpuCount        int    `json:"gpu_count"`
	GpuShape        string `json:"gpu_shape"`
	ModelName       string `json:"model_name"`
}

func (m ModelArtifact) GetName() string {
	return m.Name
}

func (m ModelArtifact) GetKey() string {
	return fmt.Sprintf("%s-%s", m.ModelName, m.GpuShape)
}

func (m ModelArtifact) GetGpuConfig() string {
	return fmt.Sprintf("%dx %s", m.GpuCount, m.GpuShape)
}

func (m ModelArtifact) GetFilterableFields() []string {
	return []string{m.Name, m.GetGpuConfig(), m.ModelName}
}
