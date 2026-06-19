package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GPUWorkloadColumns is the canonical column set for domain.GPUWorkload.
// 8 columns, ratios sum to 1.00. Node is the group key and MUST stay at
// index 1: itemKeyFrom/parentScope derive the scoped key and parent
// (GPUNode) from row[1] for grouped categories.
var GPUWorkloadColumns = GroupedSet[models.GPUWorkload]{Columns: []GroupedColumn[models.GPUWorkload]{
	{
		Title: "Name", Key: "name", Ratio: 0.20, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Name },
	},
	{
		Title: "Node", Key: "node", Ratio: 0.12,
		Render: func(k string, _ models.GPUWorkload) string { return k },
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.14, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.TenantName() },
		RenderForExport: func(realm, _ string, _ string, w models.GPUWorkload) string {
			return w.TenancyOCID(realm)
		},
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.14, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Namespace },
	},
	{
		Title: "Model", Key: "model", Ratio: 0.13,
		Render: func(_ string, w models.GPUWorkload) string { return w.Model },
	},
	{
		Title: "Runtime", Key: "runtime", Ratio: 0.13, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Runtime },
	},
	{
		Title: "GPUs", Key: "gpus", Ratio: 0.06,
		Render: func(_ string, w models.GPUWorkload) string { return strconv.Itoa(w.GPUs) },
	},
	{
		Title: "Mode", Key: "mode", Ratio: 0.08,
		Render: func(_ string, w models.GPUWorkload) string { return w.Mode },
	},
}}
