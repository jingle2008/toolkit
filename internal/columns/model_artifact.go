package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// ModelArtifactColumns is the canonical column set for domain.ModelArtifact.
// The group key is the parent BaseModel name (equals a.ModelName). The
// "Model Internal Name" column renders a.ModelName (ignores k) to match
// TUI behaviour. Ordering is name-first, key-second (Decision #4).
var ModelArtifactColumns = GroupedSet[models.ModelArtifact]{Columns: []GroupedColumn[models.ModelArtifact]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.50,
		Render: func(_ string, a models.ModelArtifact) string { return a.Name }},
	{Title: "Model Internal Name", Key: "model-internal-name", Default: true, Ratio: 0.30,
		Render: func(_ string, a models.ModelArtifact) string { return a.ModelName }},
	{Title: "GPU Config", Key: "gpu-config", Default: true, Ratio: 0.10,
		Render: func(_ string, a models.ModelArtifact) string { return a.GetGpuConfig() }},
	{Title: "TensorRT", Key: "tensorrt", Default: true, Ratio: 0.10,
		Render: func(_ string, a models.ModelArtifact) string { return a.TensorRTVersion }},
}}
