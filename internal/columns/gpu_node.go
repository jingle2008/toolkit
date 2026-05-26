package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GPUNodeColumns is the canonical column set for domain.GPUNode.
// Ordering is name-first, pool-key-second (matches TUI; Decision #4).
var GPUNodeColumns = GroupedSet[models.GPUNode]{Columns: []GroupedColumn[models.GPUNode]{
	{
		Title: "Name", Key: "name", Ratio: 0.15,
		Render: func(_ string, n models.GPUNode) string { return n.Name },
	},
	{
		Title: "Pool", Key: "pool", Ratio: 0.22,
		Render: func(k string, _ models.GPUNode) string { return k },
	},
	{
		Title: "Type", Key: "type", Ratio: 0.15,
		Render: func(_ string, n models.GPUNode) string { return n.InstanceType },
	},
	{
		Title: "Total", Key: "total", Ratio: 0.06,
		Render: func(_ string, n models.GPUNode) string { return strconv.Itoa(n.Allocatable) },
	},
	{
		Title: "Free", Key: "free", Ratio: 0.06,
		Render: func(_ string, n models.GPUNode) string { return strconv.Itoa(n.Allocatable - n.Allocated) },
	},
	{
		Title: "Healthy", Key: "healthy", Ratio: 0.06,
		Render: func(_ string, n models.GPUNode) string { return strconv.FormatBool(n.IsHealthy()) },
	},
	{
		Title: "Ready", Key: "ready", Ratio: 0.06,
		Render: func(_ string, n models.GPUNode) string { return strconv.FormatBool(n.IsReady) },
	},
	{
		Title: "Age", Key: "age", Ratio: 0.06,
		Render: func(_ string, n models.GPUNode) string { return n.Age },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.18,
		Render: func(_ string, n models.GPUNode) string { return n.GetStatus() },
	},
}}
