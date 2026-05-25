package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GpuPoolColumns is the canonical column set for domain.GpuPool.
// Default==true columns match today's CLI (Name, Shape, Size, Actual Size,
// Capacity Type, Status). AD, GPUs, OKE Managed are Default==false (opt-in).
var GpuPoolColumns = Set[models.GpuPool]{Columns: []Column[models.GpuPool]{
	{
		Title: "Name", Key: "name", Ratio: 0.22,
		Render: func(p models.GpuPool) string { return p.Name },
	},
	{
		Title: "Shape", Key: "shape", Ratio: 0.20,
		Render: func(p models.GpuPool) string { return p.Shape },
	},
	{
		Title: "AD", Key: "ad", Ratio: 0.06,
		Render: func(p models.GpuPool) string { return p.AvailabilityDomain },
	},
	{
		Title: "Size", Key: "size", Ratio: 0.06,
		Render: func(p models.GpuPool) string { return strconv.Itoa(p.Size) },
	},
	{
		Title: "Actual Size", Key: "actual-size", Ratio: 0.10,
		Render: func(p models.GpuPool) string { return strconv.Itoa(p.ActualSize) },
	},
	{
		Title: "GPUs", Key: "gpus", Ratio: 0.06,
		Render: func(p models.GpuPool) string { return strconv.Itoa(p.GetGPUs()) },
	},
	{
		Title: "OKE Managed", Key: "oke-managed", Ratio: 0.10,
		Render: func(p models.GpuPool) string { return strconv.FormatBool(p.IsOkeManaged) },
	},
	{
		Title: "Capacity Type", Key: "capacity-type", Ratio: 0.10,
		Render: func(p models.GpuPool) string { return p.CapacityType },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.10,
		Render: func(p models.GpuPool) string { return p.Status },
	},
}}
