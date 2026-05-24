package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GpuNodeColumns is the canonical column set for domain.GpuNode.
// Default==true columns: Name, Pool, Type, Age, Status.
// Default==false columns: Total, Free, Healthy, Ready (opt-in).
// Ordering is name-first, pool-key-second (matches TUI; Decision #4).
var GpuNodeColumns = GroupedSet[models.GpuNode]{Columns: []GroupedColumn[models.GpuNode]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.15,
		Render: func(_ string, n models.GpuNode) string { return n.Name }},
	{Title: "Pool", Key: "pool", Default: true, Ratio: 0.22,
		Render: func(k string, _ models.GpuNode) string { return k }},
	{Title: "Type", Key: "type", Default: true, Ratio: 0.15,
		Render: func(_ string, n models.GpuNode) string { return n.InstanceType }},
	{Title: "Total", Key: "total", Default: true, Ratio: 0.06,
		Render: func(_ string, n models.GpuNode) string { return strconv.Itoa(n.Allocatable) }},
	{Title: "Free", Key: "free", Default: true, Ratio: 0.06,
		Render: func(_ string, n models.GpuNode) string { return strconv.Itoa(n.Allocatable - n.Allocated) }},
	{Title: "Healthy", Key: "healthy", Default: true, Ratio: 0.06,
		Render: func(_ string, n models.GpuNode) string { return strconv.FormatBool(n.IsHealthy()) }},
	{Title: "Ready", Key: "ready", Default: true, Ratio: 0.06,
		Render: func(_ string, n models.GpuNode) string { return strconv.FormatBool(n.IsReady) }},
	{Title: "Age", Key: "age", Default: true, Ratio: 0.06,
		Render: func(_ string, n models.GpuNode) string { return n.Age }},
	{Title: "Status", Key: "status", Default: true, Ratio: 0.18,
		Render: func(_ string, n models.GpuNode) string { return n.GetStatus() }},
}}
