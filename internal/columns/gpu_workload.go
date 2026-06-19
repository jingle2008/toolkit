package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GPUWorkloadColumns is the canonical column set for domain.GPUWorkload.
// 10 columns, ratios sum to 1.00. Node is the group key and MUST stay at
// index 1: itemKeyFrom/parentScope derive the scoped key and parent
// (GPUNode) from row[1] for grouped categories.
var GPUWorkloadColumns = GroupedSet[models.GPUWorkload]{Columns: []GroupedColumn[models.GPUWorkload]{
	{
		Title: "Name", Key: "name", Ratio: 0.17, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Name },
	},
	{
		Title: "Node", Key: "node", Ratio: 0.11,
		Render: func(k string, _ models.GPUWorkload) string { return k },
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.13, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.TenantName() },
		RenderForExport: func(realm, _ string, _ string, w models.GPUWorkload) string {
			return w.TenancyOCID(realm)
		},
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.12, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Namespace },
	},
	{
		Title: "Model", Key: "model", Ratio: 0.11,
		Render: func(_ string, w models.GPUWorkload) string { return w.Model },
	},
	{
		Title: "Runtime", Key: "runtime", Ratio: 0.11, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Runtime },
	},
	{
		Title: "GPUs", Key: "gpus", Ratio: 0.05,
		Render: func(_ string, w models.GPUWorkload) string { return strconv.Itoa(w.GPUs) },
	},
	{
		Title: "Restarts", Key: "restarts", Ratio: 0.05,
		Render: func(_ string, w models.GPUWorkload) string { return strconv.Itoa(w.Restarts) },
	},
	{
		Title: "Age", Key: "age", Ratio: 0.05,
		Render: func(_ string, w models.GPUWorkload) string { return w.Age },
	},
	{
		Title: "Mode", Key: "mode", Ratio: 0.10,
		Render: func(_ string, w models.GPUWorkload) string { return w.Mode },
	},
}}
